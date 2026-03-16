package incident

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/argus-monitoring/argus/internal/eventbus"
	"github.com/argus-monitoring/argus/internal/model"
	"github.com/argus-monitoring/argus/pkg/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Service struct {
	DB  *gorm.DB
	Bus eventbus.EventBus
}

func NewService(db *gorm.DB, bus eventbus.EventBus) *Service {
	s := &Service{DB: db, Bus: bus}
	s.subscribe()
	return s
}

func (s *Service) subscribe() {
	_ = s.Bus.Subscribe("alert.firing", s.HandleAlertFiring)
	_ = s.Bus.Subscribe("alert.resolved", s.HandleAlertResolved)
}

func (s *Service) HandleAlertFiring(ctx context.Context, event eventbus.Event) error {
	alarmData, ok := event.Data.(model.Alarm)
	if !ok {
		return fmt.Errorf("invalid event data type")
	}

	var alarm model.Alarm
	if err := s.DB.First(&alarm, alarmData.ID).Error; err != nil {
		return err
	}

	var rule model.AlertRule
	if err := s.DB.First(&rule, alarm.AlertRuleID).Error; err != nil {
		return err
	}

	labels := map[string]string{}
	if strings.TrimSpace(alarm.Labels) != "" {
		_ = json.Unmarshal([]byte(alarm.Labels), &labels)
	}
	if len(labels) == 0 {
		labels = extractLabels(alarm.Content)
	}
	if v, ok := labels["field"]; ok && strings.TrimSpace(v) == "__count__" {
		delete(labels, "field")
	}
	if v, ok := labels["mode"]; ok && strings.TrimSpace(v) == "row_count" {
		delete(labels, "mode")
	}
	keyLabels := pickKeyLabels(labels)
	groupKey := normalizeKV(keyLabels)
	groupKey = normalizeGroupKey(groupKey)
	if strings.TrimSpace(groupKey) == "" {
		groupKey = parseGroupKey(alarm.Content)
	}
	groupKey = normalizeGroupKey(groupKey)
	dedupKey := buildDedupKey(rule.TeamID, groupKey, alarm.AlertRuleID, alarm.Fingerprint)
	legacyDedupKey := buildLegacyDedupKey(rule.TeamID, groupKey)
	now := time.Now()

	var inc model.Incident
	err := s.DB.Where("team_id = ? AND dedup_key = ? AND status IN ?", rule.TeamID, dedupKey, []model.IncidentStatus{model.IncidentStatusOpen, model.IncidentStatusAcked}).
		Order("last_activity_at desc").
		First(&inc).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if legacyDedupKey != "" && legacyDedupKey != dedupKey {
			var old model.Incident
			e2 := s.DB.Where("team_id = ? AND dedup_key = ? AND status IN ?", rule.TeamID, legacyDedupKey, []model.IncidentStatus{model.IncidentStatusOpen, model.IncidentStatusAcked}).
				Order("last_activity_at desc").
				First(&old).Error
			if e2 == nil {
				_ = s.DB.Model(&model.Incident{}).Where("id = ?", old.ID).Update("dedup_key", dedupKey).Error
				inc = old
				err = nil
			}
		}
	}
	if err != nil {
		title := buildIncidentTitle(rule.Name, keyLabels, groupKey)
		severity := mapSeverity(rule.Level)
		openedAt := alarm.StartsAt
		if openedAt.IsZero() {
			openedAt = now
		}
		tagsJSON := ""
		if len(keyLabels) > 0 {
			if b, err := json.Marshal(keyLabels); err == nil {
				tagsJSON = string(b)
			}
		}
		inc = model.Incident{
			TeamID:         rule.TeamID,
			Title:          title,
			Summary:        alarm.Content,
			Status:         model.IncidentStatusOpen,
			Severity:       severity,
			DedupKey:       dedupKey,
			GroupKey:       groupKey,
			Tags:           tagsJSON,
			OpenedAt:       openedAt,
			LastActivityAt: now,
			RootAlarmID:    &alarm.ID,
		}
		if err := s.DB.Create(&inc).Error; err != nil {
			return err
		}
	} else {
		updates := map[string]interface{}{
			"last_activity_at": now,
		}
		if inc.RootAlarmID == nil {
			updates["root_alarm_id"] = alarm.ID
		}
		if strings.TrimSpace(inc.GroupKey) == "" || inc.GroupKey == "default" {
			if strings.TrimSpace(groupKey) != "" && groupKey != "default" {
				updates["group_key"] = groupKey
			}
		}
		if strings.TrimSpace(inc.Tags) == "" && len(keyLabels) > 0 {
			if b, err := json.Marshal(keyLabels); err == nil {
				updates["tags"] = string(b)
			}
		}
		_ = s.DB.Model(&model.Incident{}).Where("id = ?", inc.ID).Updates(updates).Error
	}

	var rel model.IncidentAlarm
	err = s.DB.Where("incident_id = ? AND alarm_id = ?", inc.ID, alarm.ID).First(&rel).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			_ = s.DB.Create(&model.IncidentAlarm{
				IncidentID:   inc.ID,
				AlarmID:      alarm.ID,
				RelationType: "related",
			}).Error
		} else {
			return err
		}
	}

	logger.Log.Info("Incident updated for alarm", zap.Uint("incident_id", inc.ID), zap.Uint("alarm_id", alarm.ID))
	_ = ctx
	return nil
}

func (s *Service) HandleAlertResolved(ctx context.Context, event eventbus.Event) error {
	alarmData, ok := event.Data.(model.Alarm)
	if !ok {
		return fmt.Errorf("invalid event data type")
	}
	var alarm model.Alarm
	if err := s.DB.First(&alarm, alarmData.ID).Error; err != nil {
		return err
	}

	var rels []model.IncidentAlarm
	if err := s.DB.Where("alarm_id = ?", alarm.ID).Find(&rels).Error; err != nil {
		return err
	}
	if len(rels) == 0 {
		return nil
	}

	now := time.Now()
	for _, rel := range rels {
		var inc model.Incident
		if err := s.DB.First(&inc, rel.IncidentID).Error; err != nil {
			continue
		}
		if inc.Status == model.IncidentStatusResolved {
			continue
		}
		var firingCount int64
		if err := s.DB.Table("incident_alarms").
			Joins("JOIN alarms ON alarms.id = incident_alarms.alarm_id").
			Where("incident_alarms.incident_id = ? AND alarms.status = ?", inc.ID, model.AlarmStatusFiring).
			Count(&firingCount).Error; err != nil {
			continue
		}
		if firingCount == 0 {
			inc.Status = model.IncidentStatusResolved
			inc.ResolvedAt = &now
			inc.LastActivityAt = now
			_ = s.DB.Save(&inc).Error
		} else {
			_ = s.DB.Model(&model.Incident{}).Where("id = ?", inc.ID).Update("last_activity_at", now).Error
		}
	}
	_ = ctx
	return nil
}

func mapSeverity(level string) model.IncidentSeverity {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "critical":
		return model.IncidentSeverityCritical
	case "warning":
		return model.IncidentSeverityWarning
	case "info":
		return model.IncidentSeverityInfo
	default:
		return model.IncidentSeverityWarning
	}
}

func parseGroupKey(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Group:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Group:"))
		}
	}
	return "default"
}

func buildDedupKey(teamID uint, groupKey string, ruleID uint, fingerprint string) string {
	gk := strings.TrimSpace(groupKey)
	key := ""
	if gk != "" && gk != "default" {
		key = fmt.Sprintf("%d:rule=%d:%s", teamID, ruleID, gk)
	} else {
		key = fmt.Sprintf("%d:rule=%d:fingerprint=%s", teamID, ruleID, strings.TrimSpace(fingerprint))
	}
	if len(key) <= 255 {
		return key
	}
	h := md5.Sum([]byte(key))
	return fmt.Sprintf("%d:%s", teamID, hex.EncodeToString(h[:]))
}

func buildLegacyDedupKey(teamID uint, groupKey string) string {
	gk := strings.TrimSpace(groupKey)
	if gk == "" || gk == "default" {
		return ""
	}
	key := fmt.Sprintf("%d:%s", teamID, gk)
	if len(key) <= 255 {
		return key
	}
	h := md5.Sum([]byte(key))
	return fmt.Sprintf("%d:%s", teamID, hex.EncodeToString(h[:]))
}

func buildIncidentTitle(ruleName string, keyLabels map[string]string, groupKey string) string {
	keys := []string{"service", "app", "job", "namespace", "env", "cluster", "host", "instance", "pod", "node"}
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		if v, ok := keyLabels[k]; ok && strings.TrimSpace(v) != "" {
			parts = append(parts, fmt.Sprintf("%s=%s", k, v))
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, " ")
	}
	if strings.TrimSpace(groupKey) != "" && groupKey != "default" {
		return fmt.Sprintf("%s (%s)", ruleName, groupKey)
	}
	return ruleName
}

func normalizeGroupKey(groupKey string) string {
	gk := strings.TrimSpace(groupKey)
	if gk == "" || gk == "default" {
		return "default"
	}
	pairs := strings.Split(gk, ",")
	out := make([]string, 0, len(pairs))
	for _, kv := range pairs {
		kv = strings.TrimSpace(kv)
		if kv == "" {
			continue
		}
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			out = append(out, kv)
			continue
		}
		k := strings.TrimSpace(parts[0])
		v := strings.TrimSpace(parts[1])
		if k == "field" && v == "__count__" {
			continue
		}
		if k == "mode" && v == "row_count" {
			continue
		}
		out = append(out, fmt.Sprintf("%s=%s", k, v))
	}
	if len(out) == 0 {
		return "default"
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}

func pickKeyLabels(labels map[string]string) map[string]string {
	preferred := []string{"env", "cluster", "namespace", "service", "app", "job", "host", "instance", "pod", "node"}
	out := map[string]string{}
	for _, k := range preferred {
		if v, ok := labels[k]; ok && strings.TrimSpace(v) != "" {
			out[k] = v
		}
	}
	if len(out) > 0 {
		return out
	}
	return labels
}

func normalizeKV(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m))
	for k, v := range m {
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k == "" || v == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		v := strings.TrimSpace(m[k])
		if v == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(parts, ",")
}

func extractLabels(content string) map[string]string {
	out := map[string]string{}
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Group:") {
			group := strings.TrimSpace(strings.TrimPrefix(line, "Group:"))
			for _, kv := range strings.Split(group, ",") {
				kv = strings.TrimSpace(kv)
				if kv == "" {
					continue
				}
				parts := strings.SplitN(kv, "=", 2)
				if len(parts) != 2 {
					continue
				}
				k := strings.TrimSpace(parts[0])
				v := strings.TrimSpace(parts[1])
				if k != "" && v != "" {
					out[k] = v
				}
			}
			continue
		}
		if strings.Contains(line, "Metric:") {
			if i := strings.Index(line, "{"); i >= 0 {
				if j := strings.LastIndex(line, "}"); j > i {
					inside := line[i+1 : j]
					for k, v := range parsePromLabels(inside) {
						if _, ok := out[k]; !ok {
							out[k] = v
						}
					}
				}
			}
		}
	}
	return out
}

var promLabelRE = regexp.MustCompile(`([a-zA-Z_][a-zA-Z0-9_]*)\s*=\s*"([^"\\]*(?:\\.[^"\\]*)*)"\s*`)

func parsePromLabels(s string) map[string]string {
	out := map[string]string{}
	matches := promLabelRE.FindAllStringSubmatch(s, -1)
	for _, m := range matches {
		if len(m) != 3 {
			continue
		}
		k := strings.TrimSpace(m[1])
		v := strings.TrimSpace(m[2])
		if k == "" || v == "" {
			continue
		}
		out[k] = v
	}
	return out
}
