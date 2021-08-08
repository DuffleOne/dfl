package rpc

import (
	"encoding/json"
	"net/http"

	"github.com/cuvva/cuvva-public-go/lib/cher"
	"github.com/sirupsen/logrus"
)

func HandleError(w http.ResponseWriter, r *http.Request, err error, logger *logrus.Entry) {
	if err == nil {
		return
	}

	w.Header().Add("Content-Type", "application/json; charset=utf-8")

	if e, ok := err.(cher.E); ok {
		switch e.Code {
		case cher.BadRequest:
			logInfo(logger, err)
			w.WriteHeader(400)
		case cher.Unauthorized:
			logInfo(logger, err)
			w.WriteHeader(401)
		case cher.AccessDenied:
			logInfo(logger, err)
			w.WriteHeader(403)
		case cher.NotFound:
			logInfo(logger, err)
			w.WriteHeader(404)
		default:
			logWarn(logger, err)
			w.WriteHeader(500)
		}

		json.NewEncoder(w).Encode(e)
		return
	}

	w.WriteHeader(500)
	logWarn(logger, err)
	json.NewEncoder(w).Encode(cher.New("unknown", cher.M{"error": err.Error()}))
}

func logInfo(l *logrus.Entry, err error) {
	if l == nil {
		return
	}

	l.WithError(err).Info(err)
}

func logWarn(l *logrus.Entry, err error) {
	if l == nil {
		return
	}

	l.WithError(err).Warn(err)
}
