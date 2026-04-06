package ws

import (
	"net/http"
	"strings"
)

// NewOriginChecker returns CheckOrigin for the websocket upgrader.
// allowedCSV: from WS_ALLOWED_ORIGINS — "*" allows any; comma-separated origins; when strict is true and allowedCSV is empty, reject requests that send a non-empty Origin header.
// strict: when true and allowedCSV is empty, reject requests that send a non-empty Origin header.
func NewOriginChecker(allowedCSV string, strict bool) func(r *http.Request) bool {
	allowedCSV = strings.TrimSpace(allowedCSV)
	if allowedCSV == "*" || allowedCSV == "all" {
		return func(r *http.Request) bool { return true }
	}
	var list []string
	if allowedCSV != "" {
		for _, p := range strings.Split(allowedCSV, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				list = append(list, p)
			}
		}
	}
	return func(r *http.Request) bool {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin == "" {
			// Non-browser clients typically omit Origin.
			return true
		}
		if len(list) > 0 {
			for _, o := range list {
				if origin == o {
					return true
				}
			}
			return false
		}
		if strict {
			return false
		}
		return true
	}
}
