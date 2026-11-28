package domain

import (
	"regexp"
	"strconv"
	"strings"
)

// ScheduleChange is explicit: nil ID adds, an ID edits; omission never deletes.
type ScheduleChange struct {
	ID             *string `json:"id"`
	CronExpression string  `json:"cron_expression"`
}

var cronAtom = regexp.MustCompile(`^(\*|[0-9]+(?:-[0-9]+)?)(?:/([0-9]+))?$`)

// ValidCron validates only the documented five-field numeric grammar. #19 must
// share this contract when implementing Asia/Tokyo time matching (including the
// Vixie day/day-of-week rule); accepting syntax does not promise a future date.
func ValidCron(value string) bool {
	if len(value) == 0 || len(value) > 256 {
		return false
	}
	for _, c := range value {
		if !(c == ' ' || c == '\t' || c == '*' || c == ',' || c == '-' || c == '/' || c >= '0' && c <= '9') {
			return false
		}
	}
	fields := strings.Fields(value)
	if len(fields) != 5 {
		return false
	}
	bounds := [][2]int{{0, 59}, {0, 23}, {1, 31}, {1, 12}, {0, 7}}
	for i, field := range fields {
		lo, hi := bounds[i][0], bounds[i][1]
		for _, atom := range strings.Split(field, ",") {
			parts := cronAtom.FindStringSubmatch(atom)
			if parts == nil {
				return false
			}
			base := parts[1]
			if parts[2] != "" {
				step, err := cronNumber(parts[2])
				if err != nil || step < 1 || step > hi || base != "*" && !strings.Contains(base, "-") {
					return false
				}
			}
			if base == "*" {
				continue
			}
			ends := strings.Split(base, "-")
			start, err := cronNumber(ends[0])
			if err != nil || start < lo || start > hi {
				return false
			}
			if len(ends) == 2 {
				end, err := cronNumber(ends[1])
				if err != nil || end < start || end > hi {
					return false
				}
			}
		}
	}
	return true
}

func cronNumber(s string) (int, error) {
	s = strings.TrimLeft(s, "0")
	if s == "" {
		return 0, nil
	}
	return strconv.Atoi(s)
}
