package domain

import (
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type SourceChange struct {
	ID            *string `json:"id"`
	WatchTargetID string  `json:"watch_target_id"`
	Name          string  `json:"name"`
	URL           string  `json:"url"`
	Instruction   string  `json:"instruction"`
}

var sourceAuthority = regexp.MustCompile(`(?i)^https?://([^/?#]+)`)
var sourceHostLabel = regexp.MustCompile(`^[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)
var badSourceEscape = regexp.MustCompile(`%`)
var sourceHexPair = regexp.MustCompile(`^[0-9a-fA-F]{2}$`)

// Validate the saved string, without DNS, fetching or URL normalization.
func ValidSourceURL(value string) bool {
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > 2048 {
		return false
	}
	for _, r := range value {
		if unicode.IsSpace(r) || unicode.IsControl(r) || r == '\\' {
			return false
		}
	}
	// net/url accepts invalid percent escapes in RawQuery; check every component.
	for _, pos := range badSourceEscape.FindAllStringIndex(value, -1) {
		i := pos[0]
		if i+2 >= len(value) {
			return false
		}
		if !sourceHexPair.MatchString(value[i+1 : i+3]) {
			return false
		}
	}
	match := sourceAuthority.FindStringSubmatch(value)
	if match == nil {
		return false
	}
	u, err := url.Parse(value)
	if err != nil || u.User != nil || u.Opaque != "" || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	authority := match[1]
	if strings.ContainsAny(authority, "@%") {
		return false
	}
	host := u.Hostname()
	if strings.HasPrefix(authority, "[") {
		if net.ParseIP(host) == nil || !strings.Contains(host, ":") {
			return false
		}
	} else {
		if strings.Contains(host, ":") || len(host) > 253 {
			return false
		}
		for _, label := range strings.Split(strings.TrimSuffix(host, "."), ".") {
			if !sourceHostLabel.MatchString(label) {
				return false
			}
		}
	}
	// Require exactly host or host:port; a trailing colon is not an omitted port.
	expected := host
	if strings.HasPrefix(authority, "[") {
		expected = "[" + host + "]"
	}
	if authority != expected {
		port := u.Port()
		if authority != expected+":"+port || port == "" {
			return false
		}
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return false
		}
	}
	return true
}
