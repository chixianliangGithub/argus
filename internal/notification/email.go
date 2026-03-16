package notification

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"regexp"
	"strings"
	"time"

	"github.com/argus-monitoring/argus/internal/model"
)

type EmailSender struct {
}

func NewEmailSender() *EmailSender {
	return &EmailSender{}
}

func (s *EmailSender) Send(ctx context.Context, alarm *model.Alarm, config map[string]string) error {
	host := config["smtp_host"]
	port := config["smtp_port"]
	username := config["username"]
	password := config["password"]
	to := config["to"] // comma separated list of emails
	from := strings.TrimSpace(config["from"])
	if from == "" {
		from = username
	}
	subjectText := strings.TrimSpace(config["subject"])
	if subjectText == "" {
		subjectText = fmt.Sprintf("[Argus Alarm] %s", alarm.Status)
	}
	if strings.TrimSpace(config["level"]) != "" {
		lv := strings.TrimSpace(config["level"])
		subjectText = fmt.Sprintf("[%s] %s", strings.ToUpper(lv), subjectText)
	}
	if strings.TrimSpace(config["title"]) != "" {
		subjectText = strings.TrimSpace(config["title"])
	}
	reCtl := regexp.MustCompile(`[\r\n]+`)
	subjectText = reCtl.ReplaceAllString(subjectText, " ")
	tlsMode := strings.TrimSpace(config["tls_mode"]) // plain | starttls | tls

	if host == "" || port == "" || username == "" || password == "" || to == "" {
		return fmt.Errorf("missing email config")
	}

	auth := smtp.PlainAuth("", username, password, host)
	rawTo := strings.Split(to, ",")
	var toAddr []string
	for _, x := range rawTo {
		xx := strings.TrimSpace(x)
		if xx != "" {
			toAddr = append(toAddr, xx)
		}
	}
	if len(toAddr) == 0 {
		return fmt.Errorf("missing email recipients")
	}

	body := config["template"]
	if body == "" {
		body = fmt.Sprintf("Content: %s\nStatus: %s\nTime: %s", alarm.Content, alarm.Status, alarm.StartsAt.Format(time.RFC3339))
	}
	msg := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
		from,
		strings.Join(toAddr, ","),
		subjectText,
		body,
	))

	addr := fmt.Sprintf("%s:%s", host, port)
	if tlsMode == "" || tlsMode == "plain" {
		return smtp.SendMail(addr, auth, from, toAddr, msg)
	}

	if tlsMode == "tls" {
		dialer := &net.Dialer{}
		conn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: host})
		if err != nil {
			return err
		}
		defer conn.Close()
		c, err := smtp.NewClient(conn, host)
		if err != nil {
			return err
		}
		defer c.Quit()
		if err := c.Auth(auth); err != nil {
			return err
		}
		if err := c.Mail(from); err != nil {
			return err
		}
		for _, rcpt := range toAddr {
			if err := c.Rcpt(rcpt); err != nil {
				return err
			}
		}
		w, err := c.Data()
		if err != nil {
			return err
		}
		if _, err := w.Write(msg); err != nil {
			_ = w.Close()
			return err
		}
		return w.Close()
	}

	if tlsMode == "starttls" {
		c, err := smtp.Dial(addr)
		if err != nil {
			return err
		}
		defer c.Quit()
		if err := c.Hello("localhost"); err != nil {
			return err
		}
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(&tls.Config{ServerName: host}); err != nil {
				return err
			}
		}
		if ok, _ := c.Extension("AUTH"); ok {
			if err := c.Auth(auth); err != nil {
				return err
			}
		}
		if err := c.Mail(from); err != nil {
			return err
		}
		for _, rcpt := range toAddr {
			if err := c.Rcpt(rcpt); err != nil {
				return err
			}
		}
		w, err := c.Data()
		if err != nil {
			return err
		}
		if _, err := w.Write(msg); err != nil {
			_ = w.Close()
			return err
		}
		return w.Close()
	}

	return fmt.Errorf("unsupported tls_mode: %s", tlsMode)
}
