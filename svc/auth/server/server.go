package server

import (
	"database/sql"
	"fmt"
	"net/http"

	"dfl/lib/key"
	dflmw "dfl/lib/middleware"
	"dfl/lib/ptr"
	rpclib "dfl/lib/rpc"
	"dfl/svc/auth/server/app"
	"dfl/svc/auth/server/db"
	"dfl/svc/auth/server/rpc"

	"github.com/duo-labs/webauthn/webauthn"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	_ "github.com/lib/pq" // required for the PGSQL driver to be loaded
	"github.com/rs/cors"
	"github.com/sirupsen/logrus"
)

const privateKey = `-----BEGIN EC PRIVATE KEY-----
MIGkAgEBBDAWpmGQ3J04zCgXdYgw/o8CIsB+9aG1b/UxKP0pU0ws4JyZ7EjXvbJo
/t+HptXPs7ugBwYFK4EEACKhZANiAAQxzjcwIr8FkpP61946t7+0CE3OLY6+sTKK
8MojiLFomEpJ2MYou+SjPc7m0ZSA9Yi26Ba5QyiHNgOo6cNVQBrNrYd47dJQ4YYp
4ojMVyng1IOaN0tSO37xrr/BjcQCrEw=
-----END EC PRIVATE KEY-----`

const publicKey = `-----BEGIN PUBLIC KEY-----
MHYwEAYHKoZIzj0CAQYFK4EEACIDYgAEMc43MCK/BZKT+tfeOre/tAhNzi2OvrEy
ivDKI4ixaJhKSdjGKLvkoz3O5tGUgPWItugWuUMohzYDqOnDVUAaza2HeO3SUOGG
KeKIzFcp4NSDmjdLUjt+8a6/wY3EAqxM
-----END PUBLIC KEY-----`

type Config struct {
	Logger *logrus.Logger

	Port int    `envconfig:"port"`
	DSN  string `envconfig:"dsn"`

	PrivateKey string `envconfig:"private_key"`
	PublicKey  string `envconfig:"public_key"`

	WebAuthn WebAuthn `envconfig:"webauthn"`
}

type WebAuthn struct {
	ID          string `envconfig:"rpid"`
	Origin      string `envconfig:"rporigin"`
	DisplayName string `envconfig:"rpdisplayname"`
}

func DefaultConfig() Config {
	return Config{
		Logger: logrus.New(),
		Port:   3000,

		DSN: "postgresql://postgres@localhost/dflauth?sslmode=disable",

		PrivateKey: privateKey,
		PublicKey:  publicKey,

		WebAuthn: WebAuthn{
			ID:          "localhost",
			DisplayName: "DFL Auth",
			Origin:      "http://localhost:3000",
		},
	}
}

func Run(cfg Config) error {
	cfg.Logger.Formatter = &logrus.JSONFormatter{
		DisableTimestamp: false,
	}

	pgDb, err := sql.Open("postgres", cfg.DSN)
	if err != nil {
		return err
	}

	db := db.New(pgDb)

	sk := app.NewSK(
		key.MustParseECDSAPrivate([]byte(cfg.PrivateKey)),
		key.MustParseECDSAPublic([]byte(cfg.PublicKey)),
	)

	web, err := webauthn.New(&webauthn.Config{
		RPDisplayName: cfg.WebAuthn.DisplayName,
		RPID:          cfg.WebAuthn.ID,
		RPOrigin:      cfg.WebAuthn.Origin,
	})
	if err != nil {
		return err
	}

	app := &app.App{
		WA: web,
		SK: sk,
		DB: db,
	}

	router := chi.NewRouter()

	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)
	router.Use(cors.AllowAll().Handler)
	router.Use(dflmw.MountAppMiddleware(app))
	router.Use(dflmw.AuthMiddleware(sk.Public(), []dflmw.HTTPResource{
		{Verb: ptr.String(http.MethodGet), Path: ptr.String("/authorize")},
		{Verb: ptr.String(http.MethodGet), Path: ptr.String("/register")},
		{Verb: ptr.String(http.MethodGet), Path: ptr.String("/u2f_manage")},
		{Verb: ptr.String(http.MethodPost), Path: ptr.String("/authorize_confirm")},
		{Verb: ptr.String(http.MethodPost), Path: ptr.String("/authorize_prompt")},
		{Verb: ptr.String(http.MethodPost), Path: ptr.String("/register_confirm")},
		{Verb: ptr.String(http.MethodPost), Path: ptr.String("/register_prompt")},
		{Verb: ptr.String(http.MethodPost), Path: ptr.String("/token")},
	}))

	// Internal
	router.Get("/register", wrap(rpc.RegisterGet))
	router.Get("/u2f_manage", wrap(rpc.U2FManageGet))
	router.Post("/create_client", wrap(rpc.CreateClient))
	router.Post("/create_key_confirm", wrap(rpc.CreateKeyConfirm))
	router.Post("/create_key_prompt", wrap(rpc.CreateKeyPrompt))
	router.Post("/delete_key", wrap(rpc.DeleteKey))
	router.Post("/list_u2f_keys", wrap(rpc.ListU2FKeys))
	router.Post("/register_confirm", wrap(rpc.RegisterConfirm))
	router.Post("/register_prompt", wrap(rpc.RegisterPrompt))
	router.Post("/sign_key_confirm", wrap(rpc.SignKeyConfirm))
	router.Post("/sign_key_prompt", wrap(rpc.SignKeyPrompt))
	router.Post("/whoami", wrap(rpc.WhoAmI))

	// OAuth
	router.Get("/authorize", wrap(rpc.AuthorizeGet))
	router.Post("/authorize_confirm", wrap(rpc.AuthorizeConfirm))
	router.Post("/authorize_prompt", wrap(rpc.AuthorizePrompt))
	router.Post("/token", wrap(rpc.Token))

	cfg.Logger.Infof("Server running on port %d", cfg.Port)
	return http.ListenAndServe(fmt.Sprintf(":%d", cfg.Port), router)
}

func wrap(fn func(*app.App, http.ResponseWriter, *http.Request) error) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		err := fn(r.Context().Value(dflmw.AppContext).(*app.App), w, r)
		if err != nil {
			rpclib.HandleError(w, r, err)
			return
		}
	}
}
