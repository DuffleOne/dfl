package auth

import (
	"strings"
	"unicode"

	"github.com/cuvva/cuvva-public-go/lib/cher"
)

type Auth interface {
	Check(authorization string) (*AuthUser, error)
}

type Handler interface {
	Type() string
	Check(token string) (*AuthUser, error)
}

type Handlers []Handler

func Create(hn []Handler) Handlers {
	return Handlers(hn)
}

// GetByKind returns the first handler where the given kind matches
// the declared Type of the handler, as used as the first token in the
// Authorization header.
func (h Handlers) getByKind(kind string) Handler {
	for _, hn := range h {
		if hn.Type() == kind {
			return hn
		}
	}

	return nil
}

// Check accepts input from the HTTP Authorization header, determines the
// auth scheme used and applies that to the token.
func (h Handlers) Check(authorizationHeader string) (*AuthUser, error) {
	if authorizationHeader == "" {
		return nil, nil
	}

	kind, token := splitFirst(authorizationHeader, unicode.IsSpace)

	hn := h.getByKind(strings.ToLower(kind))
	if hn == nil {
		return nil, cher.New(cher.Unauthorized, nil, cher.New("unknown_authorization_type", nil))
	}

	return hn.Check(token)
}

func splitFirst(s string, fn func(rune) bool) (t, r string) {
	i := strings.IndexFunc(s, fn)
	if i == -1 {
		t = s
	} else {
		t = s[:i]
		r = s[i+1:]
	}

	return
}
