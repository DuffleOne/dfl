package auth

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/cuvva/cuvva-public-go/lib/cher"
	"github.com/cuvva/cuvva-public-go/lib/crpc"
)

// JSONError will encode the given error as JSON to the client with a HTTP 401 status code.
func JSONError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)

	e := json.NewEncoder(w)

	if err, ok := err.(cher.E); ok {
		e.Encode(err)
	} else {
		e.Encode(cher.New(cher.Unauthorized, nil, cher.New(cher.Unknown, cher.M{"error": err})))
	}
}

func Middleware(hns Auth) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hdr := r.Header.Get("Authorization")

			user, err := hns.Check(hdr)
			if err != nil {
				JSONError(w, err)
				return
			}

			r = SetUser(r, user)
			next.ServeHTTP(w, r)
		})
	}
}

type contextKey string

const userKey contextKey = "user"

// GetUser returns the User as extracted by the authentication handler
// from the requests context, or nil if not set.
func GetUser(r *http.Request) *AuthUser {
	return GetUserContext(r.Context())
}

// GetUserContext returns the User as extracted from the a context object,
// or nil if not set.
func GetUserContext(ctx context.Context) *AuthUser {
	if u, ok := ctx.Value(userKey).(*AuthUser); ok {
		return u
	}

	return nil
}

// SetUser sets the user within a requests context, it will
// overwrite any previously set value.
func SetUser(r *http.Request, user *AuthUser) *http.Request {
	return r.WithContext(SetUserContext(r.Context(), user))
}

// SetUserContext sets the user within a context, it will
// overwrite any previously set value.
func SetUserContext(ctx context.Context, user *AuthUser) context.Context {
	return context.WithValue(ctx, userKey, user)
}

// UnsafeNoAuthentication allows all requests, regardless of authentication.
func UnsafeNoAuthentication(next crpc.HandlerFunc) crpc.HandlerFunc {
	return func(res http.ResponseWriter, req *crpc.Request) error {
		return next(res, req)
	}
}

// AllowAllAuthenticated allows only authenticated requests.
func AllowAllAuthenticated(next crpc.HandlerFunc) crpc.HandlerFunc {
	return func(res http.ResponseWriter, req *crpc.Request) error {
		user := GetUserContext(req.Context())
		if user == nil {
			return cher.New(cher.Unauthorized, nil)
		}

		return next(res, req)
	}
}
