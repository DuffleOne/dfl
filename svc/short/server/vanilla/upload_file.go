package vanilla

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	authlib "dfl/lib/auth"
	"dfl/lib/rpc"
	"dfl/svc/short"
	"dfl/svc/short/server/app"

	"github.com/cuvva/cuvva-public-go/lib/cher"
)

// UploadFile is an RPC handler for uploading a file
func UploadFile(a *app.App, w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	authUser := authlib.GetUserContext(ctx)
	if !authUser.Can("short:upload") {
		return cher.New(cher.AccessDenied, nil)
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		return err
	}
	defer file.Close()

	var name = &header.Filename

	fileName := r.PostFormValue("name")
	if fileName != "" {
		name = &fileName
	}

	var buf bytes.Buffer
	io.Copy(&buf, file)

	res, err := a.UploadFile(ctx, authUser.ID, &short.CreateFileRequest{
		File: buf,
		Name: name,
	})
	if err != nil {
		return err
	}

	accept := r.Header.Get("Accept")

	if strings.Contains(accept, "text/plain") {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(res.URL))
	} else {
		return rpc.WriteOut(w, res)
	}

	return nil
}
