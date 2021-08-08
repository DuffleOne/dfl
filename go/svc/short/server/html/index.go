package html

import (
	"net/http"

	"dfl/svc/short/server/app"
)

func Index(_ *app.App, w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Content-Type", "") // Needed for redirect to work
	http.Redirect(w, r, "https://duffle.one", http.StatusMovedPermanently)

	return nil
}
