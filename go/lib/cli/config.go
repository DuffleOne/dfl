package cli

type Config struct {
	AuthURL  string `envconfig:"AUTH_API_URL" required:"true" default:"https://api.duffle.one/1/auth"`
	AuthUI   string `envconfig:"AUTH_UI_URL" required:"true" default:"https://auth.duffle.one"`
	ShortURL string `envconfig:"SHORT_API_URL" required:"false" default:"https://api.duffle.one/1/short"`
	ShortUI  string `envconfig:"SHORT_UI_URL" required:"false" default:"https://dfl.mn"`
}
