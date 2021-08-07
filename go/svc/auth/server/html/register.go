package html

import (
	"net/http"

	"dfl/lib/rpc"
	"dfl/svc/auth/server/app"
)

func Register(a *app.App, w http.ResponseWriter, r *http.Request) error {
	return rpc.QuickTemplate(w, map[string]interface{}{
		"title":          "Register",
		"activeRegister": true,
	}, []string{
		"./resources/auth/register.html",
		"./resources/auth/_nav.html",
		"./resources/auth/layouts/root.html",
	})
}
