package handler

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/argus-monitoring/argus/internal/model"
	"github.com/gin-gonic/gin"
)

type ServiceHandler struct{}

type serviceOverview struct {
	Service         string   `json:"service"`
	App             string   `json:"app"`
	HealthScore     int      `json:"health_score"`
	Status          string   `json:"status"`
	CriticalFiring  int      `json:"critical_firing"`
	WarningFiring   int      `json:"warning_firing"`
	InfoFiring      int      `json:"info_firing"`
	Firing          int      `json:"firing"`
	UnclaimedFiring int      `json:"unclaimed_firing"`
	OpenIncidents   int      `json:"open_incidents"`
	AckedIncidents  int      `json:"acked_incidents"`
	AlarmTrend      []int    `json:"alarm_trend"`
	IncidentTrend   []int    `json:"incident_trend"`
	Buckets         []string `json:"buckets"`
}

func parseWindow(v string) time.Duration {
	s := strings.TrimSpace(v)
	if s == "" {
		return 24 * time.Hour
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return 24 * time.Hour
	}
	if d > 7*24*time.Hour {
		return 7 * 24 * time.Hour
	}
	return d
}

func buildHourlyBuckets(now time.Time, window time.Duration) ([]time.Time, []string) {
	end := now.Truncate(time.Hour).Add(time.Hour)
	start := end.Add(-window).Truncate(time.Hour)
	if start.After(end) {
		start = end.Add(-24 * time.Hour)
	}
	var ts []time.Time
	for t := start; t.Before(end); t = t.Add(time.Hour) {
		ts = append(ts, t)
	}
	labels := make([]string, 0, len(ts))
	for _, t := range ts {
		labels = append(labels, t.Format(time.RFC3339))
	}
	return ts, labels
}

func healthScore(critical, warning, openInc, unclaimed int) (int, string) {
	score := 100 - (critical*30 + warning*10 + openInc*20 + unclaimed*5)
	if score < 0 {
		score = 0
	}
	status := "healthy"
	if score < 70 {
		status = "down"
	} else if score < 90 {
		status = "degraded"
	}
	return score, status
}

func (h *ServiceHandler) Overview(c *gin.Context) {
	window := parseWindow(c.Query("window"))
	now := time.Now()
	bucketTimes, bucketLabels := buildHourlyBuckets(now, window)
	startTime := bucketTimes[0]

	var teamID uint
	if raw := strings.TrimSpace(c.Query("team_id")); raw != "" {
		if id, err := strconv.Atoi(raw); err == nil && id > 0 {
			teamID = uint(id)
		}
	}
	q := strings.TrimSpace(c.Query("q"))

	serviceExpr := "CASE WHEN alarms.service <> '' THEN alarms.service ELSE alarms.app END"
	appExpr := "CASE WHEN alarms.service <> '' THEN alarms.app ELSE '' END"

	type alarmAggRow struct {
		Service string `json:"service"`
		App     string `json:"app"`
		Level   string `json:"level"`
		Cnt     int64  `json:"cnt"`
	}
	alarmAgg := model.DB.Table("alarms").
		Select(serviceExpr+" as service, "+appExpr+" as app, alert_rules.level as level, count(*) as cnt").
		Joins("LEFT JOIN alert_rules ON alert_rules.id = alarms.alert_rule_id").
		Where("alarms.status = ?", model.AlarmStatusFiring).
		Where(serviceExpr + " <> ''").
		Group(serviceExpr + ", " + appExpr + ", alert_rules.level")
	if teamID > 0 {
		alarmAgg = alarmAgg.Where("alert_rules.team_id = ?", teamID)
	}
	if q != "" {
		like := "%" + q + "%"
		alarmAgg = alarmAgg.Where("alarms.service LIKE ? OR alarms.app LIKE ?", like, like)
	}
	var alarmRows []alarmAggRow
	_ = alarmAgg.Find(&alarmRows).Error

	type unclaimedRow struct {
		Service string `json:"service"`
		App     string `json:"app"`
		Cnt     int64  `json:"cnt"`
	}
	unclaimedAgg := model.DB.Table("alarms").
		Select(serviceExpr+" as service, "+appExpr+" as app, count(*) as cnt").
		Joins("LEFT JOIN alert_rules ON alert_rules.id = alarms.alert_rule_id").
		Where("alarms.status = ?", model.AlarmStatusFiring).
		Where("alarms.claimed_by IS NULL").
		Where(serviceExpr + " <> ''").
		Group(serviceExpr + ", " + appExpr)
	if teamID > 0 {
		unclaimedAgg = unclaimedAgg.Where("alert_rules.team_id = ?", teamID)
	}
	if q != "" {
		like := "%" + q + "%"
		unclaimedAgg = unclaimedAgg.Where("alarms.service LIKE ? OR alarms.app LIKE ?", like, like)
	}
	var unclaimedRows []unclaimedRow
	_ = unclaimedAgg.Find(&unclaimedRows).Error

	type incAggRow struct {
		Service string `json:"service"`
		App     string `json:"app"`
		Status  string `json:"status"`
		Cnt     int64  `json:"cnt"`
	}
	incAgg := model.DB.Table("incidents").
		Select(serviceExpr + " as service, " + appExpr + " as app, incidents.status as status, count(*) as cnt").
		Joins("LEFT JOIN alarms ON alarms.id = incidents.root_alarm_id").
		Where(serviceExpr + " <> ''").
		Group(serviceExpr + ", " + appExpr + ", incidents.status")
	if teamID > 0 {
		incAgg = incAgg.Where("incidents.team_id = ?", teamID)
	}
	if q != "" {
		like := "%" + q + "%"
		incAgg = incAgg.Where("alarms.service LIKE ? OR alarms.app LIKE ?", like, like)
	}
	var incRows []incAggRow
	_ = incAgg.Find(&incRows).Error

	type logRow struct {
		TS      time.Time `json:"ts"`
		Service string    `json:"service"`
		App     string    `json:"app"`
	}
	logQ := model.DB.Table("alert_logs").
		Select("alert_logs.created_at as ts, "+serviceExpr+" as service, "+appExpr+" as app").
		Joins("LEFT JOIN alarms ON alarms.id = alert_logs.alarm_id").
		Joins("LEFT JOIN alert_rules ON alert_rules.id = alert_logs.alert_rule_id").
		Where("alert_logs.status = ?", "firing").
		Where("alert_logs.created_at >= ?", startTime).
		Where(serviceExpr + " <> ''")
	if teamID > 0 {
		logQ = logQ.Where("alert_rules.team_id = ?", teamID)
	}
	if q != "" {
		like := "%" + q + "%"
		logQ = logQ.Where("alarms.service LIKE ?", like)
	}
	var logRows []logRow
	_ = logQ.Find(&logRows).Error

	type incTrendRow struct {
		TS      time.Time `json:"ts"`
		Service string    `json:"service"`
		App     string    `json:"app"`
	}
	incTrendQ := model.DB.Table("incidents").
		Select("incidents.opened_at as ts, "+serviceExpr+" as service, "+appExpr+" as app").
		Joins("LEFT JOIN alarms ON alarms.id = incidents.root_alarm_id").
		Where("incidents.opened_at >= ?", startTime).
		Where(serviceExpr + " <> ''")
	if teamID > 0 {
		incTrendQ = incTrendQ.Where("incidents.team_id = ?", teamID)
	}
	if q != "" {
		like := "%" + q + "%"
		incTrendQ = incTrendQ.Where("alarms.service LIKE ?", like)
	}
	var incTrendRows []incTrendRow
	_ = incTrendQ.Find(&incTrendRows).Error

	type svcKey struct {
		Service string
		App     string
	}
	svcs := map[svcKey]*serviceOverview{}
	getSvc := func(service, app string) *serviceOverview {
		k := svcKey{Service: service, App: app}
		if v, ok := svcs[k]; ok {
			return v
		}
		v := &serviceOverview{
			Service:       service,
			App:           app,
			AlarmTrend:    make([]int, len(bucketTimes)),
			IncidentTrend: make([]int, len(bucketTimes)),
			Buckets:       bucketLabels,
		}
		svcs[k] = v
		return v
	}

	for _, r := range alarmRows {
		s := getSvc(strings.TrimSpace(r.Service), strings.TrimSpace(r.App))
		n := int(r.Cnt)
		if r.Level == "critical" {
			s.CriticalFiring += n
		} else if r.Level == "warning" {
			s.WarningFiring += n
		} else if r.Level == "info" {
			s.InfoFiring += n
		}
		s.Firing += n
	}
	for _, r := range unclaimedRows {
		s := getSvc(strings.TrimSpace(r.Service), strings.TrimSpace(r.App))
		s.UnclaimedFiring += int(r.Cnt)
	}
	for _, r := range incRows {
		s := getSvc(strings.TrimSpace(r.Service), strings.TrimSpace(r.App))
		if r.Status == string(model.IncidentStatusOpen) {
			s.OpenIncidents += int(r.Cnt)
		} else if r.Status == string(model.IncidentStatusAcked) {
			s.AckedIncidents += int(r.Cnt)
		}
	}

	bucketMap := map[time.Time]int{}
	for i := 0; i < len(bucketTimes); i++ {
		bucketMap[bucketTimes[i]] = i
	}
	for _, r := range logRows {
		i, ok := bucketMap[r.TS.Truncate(time.Hour)]
		if !ok {
			continue
		}
		service := strings.TrimSpace(r.Service)
		app := strings.TrimSpace(r.App)
		if service == "" {
			continue
		}
		s := getSvc(service, app)
		s.AlarmTrend[i]++
	}
	for _, r := range incTrendRows {
		i, ok := bucketMap[r.TS.Truncate(time.Hour)]
		if !ok {
			continue
		}
		service := strings.TrimSpace(r.Service)
		app := strings.TrimSpace(r.App)
		if service == "" {
			continue
		}
		s := getSvc(service, app)
		s.IncidentTrend[i]++
	}

	outs := make([]serviceOverview, 0, len(svcs))
	for _, v := range svcs {
		score, st := healthScore(v.CriticalFiring, v.WarningFiring, v.OpenIncidents, v.UnclaimedFiring)
		v.HealthScore = score
		v.Status = st
		outs = append(outs, *v)
	}
	rank := map[string]int{"down": 3, "degraded": 2, "healthy": 1}
	sort.Slice(outs, func(i, j int) bool {
		ri := rank[outs[i].Status]
		rj := rank[outs[j].Status]
		if ri != rj {
			return ri > rj
		}
		return outs[i].HealthScore < outs[j].HealthScore
	})

	var changes []model.AuditLog
	_ = model.DB.Model(&model.AuditLog{}).Order("created_at desc").Limit(20).Find(&changes).Error

	c.JSON(http.StatusOK, gin.H{
		"window":   window.String(),
		"services": outs,
		"changes":  changes,
	})
}

func (h *ServiceHandler) Summary(c *gin.Context) {
	service := strings.TrimSpace(c.Param("service"))
	if service == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "service is required"})
		return
	}
	window := parseWindow(c.Query("window"))
	now := time.Now()
	start := now.Add(-window)

	type topRuleRow struct {
		RuleName string `json:"rule_name"`
		Cnt      int64  `json:"cnt"`
	}
	var topRules []topRuleRow
	_ = model.DB.Table("alarms").
		Select("alert_rules.name as rule_name, count(*) as cnt").
		Joins("LEFT JOIN alert_rules ON alert_rules.id = alarms.alert_rule_id").
		Where("alarms.status = ?", model.AlarmStatusFiring).
		Where("(alarms.service = ? OR (alarms.service = '' AND alarms.app = ?))", service, service).
		Group("alert_rules.name").
		Order("cnt desc").
		Limit(10).
		Find(&topRules).Error

	type alarmRow struct {
		model.Alarm
		RuleName string `json:"rule_name"`
		Level    string `json:"level"`
	}
	var alarms []alarmRow
	_ = model.DB.Table("alarms").
		Select("alarms.*, alert_rules.name as rule_name, alert_rules.level as level").
		Joins("LEFT JOIN alert_rules ON alert_rules.id = alarms.alert_rule_id").
		Where("(alarms.service = ? OR (alarms.service = '' AND alarms.app = ?))", service, service).
		Order("alarms.updated_at desc").
		Limit(30).
		Find(&alarms).Error

	type incidentRow struct {
		model.Incident
	}
	var incidents []incidentRow
	_ = model.DB.Table("incidents").
		Select("incidents.*").
		Joins("LEFT JOIN alarms ON alarms.id = incidents.root_alarm_id").
		Where("(alarms.service = ? OR (alarms.service = '' AND alarms.app = ?))", service, service).
		Order("incidents.last_activity_at desc").
		Limit(30).
		Find(&incidents).Error

	type logRow struct {
		model.AlertLog
		Service string `json:"service"`
		App     string `json:"app"`
	}
	var logs []logRow
	_ = model.DB.Table("alert_logs").
		Select("alert_logs.*, alarms.service as service, alarms.app as app").
		Joins("LEFT JOIN alarms ON alarms.id = alert_logs.alarm_id").
		Where("(alarms.service = ? OR (alarms.service = '' AND alarms.app = ?))", service, service).
		Where("alert_logs.created_at >= ?", start).
		Order("alert_logs.created_at desc").
		Limit(200).
		Find(&logs).Error

	var changes []model.AuditLog
	_ = model.DB.Model(&model.AuditLog{}).Order("created_at desc").Limit(20).Find(&changes).Error

	c.JSON(http.StatusOK, gin.H{
		"service":   service,
		"window":    window.String(),
		"top_rules": topRules,
		"alarms":    alarms,
		"incidents": incidents,
		"logs":      logs,
		"changes":   changes,
	})
}
