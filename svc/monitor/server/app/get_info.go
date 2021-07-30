package app

// I'm not particularly proud of this
var prodValues = map[string]struct {
	host     string
	scheme   string
	validate bool
}{
	"synclounge": {
		host:     "synclounge",
		scheme:   "http",
		validate: true,
	},
	"dfl-auth": {
		host:     "auth",
		scheme:   "http",
		validate: true,
	},
	"dfl-short": {
		host:     "short",
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
