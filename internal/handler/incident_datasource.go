package handler

import (
	"time"

	"github.com/argus-monitoring/argus/internal/model"
)

type incidentDataNameCandidate struct {
	ID             uint                 `json:"id"`
	Name           string               `json:"name"`
	DataSourceID   uint                 `json:"data_source_id"`
	DataSourceName string               `json:"data_source_name"`
	DataSourceType model.DataSourceType `json:"data_source_type"`
	AlarmCount     int                  `json:"alarm_count"`
	LastAt         time.Time            `json:"last_at"`
}

func getIncidentDataNameCandidatesByAlarmIDs(alarmIDs []uint) ([]incidentDataNameCandidate, uint) {
	if len(alarmIDs) == 0 {
		return nil, 0
	}
	var rows []incidentDataNameCandidate
	_ = model.DB.Table("alarms").
		Select("data_names.id as id, data_names.name as name, data_sources.id as data_source_id, data_sources.name as data_source_name, data_sources.type as data_source_type, count(*) as alarm_count, max(alarms.created_at) as last_at").
		Joins("left join alert_rules on alert_rules.id = alarms.alert_rule_id").
		Joins("left join data_names on data_names.id = alert_rules.data_name_id").
		Joins("left join data_sources on data_sources.id = data_names.data_source_id").
		Where("alarms.id in ?", alarmIDs).
		Group("data_names.id, data_names.name, data_sources.id, data_sources.name, data_sources.type").
		Order("alarm_count desc, last_at desc").
		Find(&rows).Error
	if len(rows) == 0 {
		return nil, 0
	}
	return rows, rows[0].ID
}
