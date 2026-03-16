package notification

import (
	"fmt"
)

var senderRegistry = make(map[string]func() AlarmSender)

func RegisterSender(name string, factory func() AlarmSender) {
	senderRegistry[name] = factory
}

func GetSender(channelType string) (AlarmSender, error) {
	if factory, ok := senderRegistry[channelType]; ok {
		return factory(), nil
	}

	switch channelType {
	case "dingtalk":
		return NewDingTalkSender(), nil
	case "feishu":
		return NewFeishuSender(), nil
	case "wechat":
		return NewWeChatSender(), nil
	case "slack":
		return NewSlackSender(), nil
	case "email":
		return NewEmailSender(), nil
	case "webhook":
		return NewWebhookSender(), nil
	default:
		return nil, fmt.Errorf("unknown channel type: %s", channelType)
	}
}
