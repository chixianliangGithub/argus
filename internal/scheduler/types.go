package scheduler

type AlertTask struct {
	RuleID uint `json:"rule_id"`
	Force  bool `json:"force,omitempty"`
}
