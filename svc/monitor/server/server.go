package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"dfl/svc/monitor/server/app"
	"dfl/svc/monitor/server/lib/cachet"

	"github.com/alexliesenfeld/health"
	"github.com/alexliesenfeld/health/middleware"
	cachetSDK "github.com/andygrunwald/cachet"
	"github.com/cuvva/cuvva-public-go/lib/config"
	"github.com/sirupsen/logrus"
)

type Config struct {
	Logger *logrus.Logger
	Server config.Server `envconfig:"server"`

	Debug bool `envconfig:"debug"`

	CachetURL string `envconfig:"cachet_url"`
	CachetKey string `envconfig:"cachet_key"`
}

func DefaultConfig() Config {
	return Config{
		Logger: logrus.New(),
		Server: config.Server{
			Addr:     "127.0.0.1:3000",
			Graceful: 5,
		},

		Debug: true,

		CachetURL: "https://status.dfl.mn",
		CachetKey: "",
	}
}

func Run(cfg Config) error {
	cfg.Logger.Formatter = &logrus.JSONFormatter{
		DisableTimestamp: false,
	}

	cachetClient, err := cachetSDK.NewClient(cfg.CachetURL, nil)
	if err != nil {
		return fmt.Errorf("cannot make cachet client: %w", err)
	}

	_, _, err = cachetClient.General.Ping()
	if err != nil {
		return fmt.Errorf("cannot ping cachet: %w", err)
	}

	cachetClient.Authentication.SetTokenAuth(cfg.CachetKey)

	client := &http.Client{
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout: 10 * time.Second,
			}).Dial,
		},
	}

	clientNoValidate := &http.Client{
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout: 10 * time.Second,
			}).Dial,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	app := &app.App{
		Logger: cfg.Logger,
		CachetNames: map[string]string{
			"overseerr":  "Overseerr",
			"synclounge": "Synclounge",
			"dfl-auth":   "Auth",
			"dfl-short":  "Short",
		},
		Cachet:           &cachet.Client{Client: cachetClient},
		Client:           client,
		ClientNoValidate: clientNoValidate,
	}

	if cfg.Debug {
		cfg.Logger.Info("setting debug ON")
	} else {
		cfg.Logger.Info("setting debug OFF")
		cfg.Logger.SetLevel(logrus.WarnLevel)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := app.CacheCachet(ctx); err != nil {
		return err
	}

	checker := app.Run(ctx)

	handler := health.NewHandler(checker,
		health.WithMiddleware(
			middleware.BasicLogger(),
		),
	)

	http.Handle("/health", handler)

	cfg.Logger.Infof("Server running")

	return http.ListenAndServe(cfg.Server.Addr, nil)
}
