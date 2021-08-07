package auth

import (
	"strings"
)

func Can(requested string, scopes string) bool {
	actions := strings.Fields(requested)
	allowed := make(map[string]struct{})

	for _, scope := range strings.Fields(scopes) {
		if scope == "*:*" {
			return true
		}

		if strings.HasSuffix(scope, ":*") {
			allowed[strings.TrimSuffix(scope, ":*")] = struct{}{}
		} else {
			allowed[scope] = struct{}{}
		}
	}

	for _, scope := range actions {
		parts := strings.Split(scope, ":")

		_, directMatch := allowed[scope]
		_, categoryMatch := allowed[parts[0]]

		if !directMatch && !categoryMatch {
			return false
		}
	}

	return true
}
