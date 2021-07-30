package app

import (
	"net/http"

	"dfl/svc/monitor/server/lib/cachet"

	"github.com/sirupsen/logrus"
)

// App is a struct for the app methods to attach to
type App struct {
	Logger *logrus.Logger

	Cache       *Cache
	CachetNames map[string]string

	Client           *http.Client
	ClientNoValidate *http.Client
	Cachet           *cachet.Client
}
