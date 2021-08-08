package html

import (
	"bytes"
	_ "embed"
	"net/http"
	"time"

	"dfl/svc/short/server/app"
)

//go:embed robots.txt
var fileContent []byte

func Robots(_ *app.App, w http.ResponseWriter, r *http.Request) error {
	modTime, err := time.Parse(time.RFC3339, "2019-10-02T12:00:00Z")
	if err != nil {
		return err
	}

	reader := bytes.NewReader(fileContent)

	http.ServeContent(w, r, "robots.txt", modTime, reader)

	return nil
}
