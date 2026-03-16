package datasource

import (
	"fmt"
	"regexp"
	"strings"
)

func InjectSQLTimeFilter(query string, timeField string, expr string) string {
	q := strings.TrimSpace(query)
	if q == "" || strings.TrimSpace(timeField) == "" || strings.TrimSpace(expr) == "" {
		return q
	}
	reOrder := regexp.MustCompile(`(?i)\border\s+by\b`)
	reLimit := regexp.MustCompile(`(?i)\blimit\b`)
	reGroup := regexp.MustCompile(`(?i)\bgroup\s+by\b`)
	reHaving := regexp.MustCompile(`(?i)\bhaving\b`)
	reSettings := regexp.MustCompile(`(?i)\bsettings\b`)
	reWhere := regexp.MustCompile(`(?i)\bwhere\b`)

	cut := len(q)
	if loc := reOrder.FindStringIndex(q); loc != nil {
		cut = loc[0]
	}
	if loc := reLimit.FindStringIndex(q); loc != nil && loc[0] < cut {
		cut = loc[0]
	}
	if loc := reGroup.FindStringIndex(q); loc != nil && loc[0] < cut {
		cut = loc[0]
	}
	if loc := reHaving.FindStringIndex(q); loc != nil && loc[0] < cut {
		cut = loc[0]
	}
	if loc := reSettings.FindStringIndex(q); loc != nil && loc[0] < cut {
		cut = loc[0]
	}
	main := strings.TrimSpace(q[:cut])
	suffix := q[cut:]

	cond := fmt.Sprintf("%s >= %s", timeField, expr)
	if reWhere.FindStringIndex(main) != nil {
		main = main + " AND " + cond
	} else {
		main = main + " WHERE " + cond
	}
	if suffix != "" {
		return main + " " + strings.TrimLeft(suffix, " \t\r\n")
	}
	return main
}
