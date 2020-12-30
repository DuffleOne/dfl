package rpc

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	authlib "dfl/lib/auth"
	"dfl/lib/cher"
	"dfl/lib/rpc"
	"dfl/svc/short"
	"dfl/svc/short/server/app"
)

// UploadFile is an RPC handler for uploading a file
func UploadFile(a *app.App) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		authUser := ctx.Value(authlib.UserContextKey).(authlib.AuthUser)
		if !authUser.Can("short:upload") && !authUser.Can("short:admin") {
			rpc.HandleError(w, r, cher.New(cher.AccessDenied, nil))
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			rpc.HandleError(w, r, err)
			return
		}
		defer file.Close()

		var name = &header.Filename

		fileName := r.PostFormValue("name")
		if fileName != "" {
			name = &fileName
		}

		var buf bytes.Buffer
		io.Copy(&buf, file)

		res, err := a.UploadFile(ctx, authUser.Username, &short.CreateFileRequest{
			File: buf,
			Name: name,
		})
		if err != nil {
			rpc.HandleError(w, r, err)
			return
		}

		accept := r.Header.Get("Accept")

		if strings.Contains(accept, "text/plain") {
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte(res.URL))
		} else {
			rpc.WriteOut(w, r, res)
		}
	}
}