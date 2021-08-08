package cli

type Config struct {
	AuthURL  string `envconfig:"AUTH_URL" required:"true" default:"https://auth.dfl.mn"`
	ShortURL string `envconfig:"SHORT_URL" required:"false" default:"https://dfl.mn"`
}
