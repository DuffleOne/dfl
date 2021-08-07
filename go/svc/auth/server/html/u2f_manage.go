package html

import (
	"net/http"

	"dfl/lib/rpc"
	"dfl/svc/auth/server/app"
)

func U2FManage(a *app.App, w http.ResponseWriter, r *http.Request) error {
	return rpc.QuickTemplate(w, map[string]interface{}{
		"title":           "Manage U2F",
		"activeManageU2F": true,
	}, []string{
		"./resources/auth/u2f_manage.html",
		"./resources/auth/_nav.html",
		"./resources/auth/layouts/root.html",
	})
}
