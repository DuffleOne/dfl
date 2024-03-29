package app

// I'm not particularly proud of this
var prodValues = map[string]struct {
	host     string
	scheme   string
	validate bool
}{
	"synclounge": {
		host:     "synclounge:8088",
		scheme:   "http",
		validate: true,
	},
	"dfl-auth": {
		host:     "auth/system/health",
		scheme:   "http",
		validate: true,
	},
	"dfl-short": {
		host:     "short/system/health",
		scheme:   "http",
		validate: true,
	},
}

func (a *App) Get(name, host, scheme string, validate bool) (string, string, bool) {
	if v, ok := prodValues[name]; ok && !a.Debug {
		return v.host, v.scheme, v.validate
	}

	return host, scheme, validate
}
