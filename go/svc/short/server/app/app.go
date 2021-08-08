package app

import (
	"dfl/lib/cache"
	"dfl/svc/short/server/db"
	"dfl/svc/short/server/lib/storageproviders"

	"github.com/nishanths/go-xkcd/v2"
	"github.com/sirupsen/logrus"
	hashids "github.com/speps/go-hashids"
)

// App is a struct for the App and it's handlers
type App struct {
	Logger *logrus.Entry

	DB     *db.DB
	SP     storageproviders.StorageProvider
	Hasher *hashids.HashID
	Redis  *cache.Cache

	XKCD *xkcd.Client

	RootURL string
}
