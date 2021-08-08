package server

import (
	"database/sql"
	"net/http"

	"dfl/lib/auth"
	"dfl/lib/key"
	rpclib "dfl/lib/rpc"
	"dfl/svc/auth/server/app"
	"dfl/svc/auth/server/db"
	"dfl/svc/auth/server/rpc"

	"github.com/cuvva/cuvva-public-go/lib/clog"
	"github.com/cuvva/cuvva-public-go/lib/config"
	"github.com/duo-labs/webauthn/webauthn"
	_ "github.com/lib/pq" // required for the PGSQL driver to be loaded
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
	Logging clog.Config   `envconfig:"logging"`
	Server  config.Server `envconfig:"server"`

	DSN string `envconfig:"dsn"`

	PrivateKey string `envconfig:"private_key"`
	PublicKey  string `envconfig:"public_key"`
	JWTIssuer  string `envconfig:"jwt_issuer"`

	WebAuthn WebAuthn `envconfig:"webauthn"`
}

type WebAuthn struct {
	ID          string `envconfig:"rpid"`
	Origin      string `envconfig:"rporigin"`
	DisplayName string `envconfig:"rpdisplayname"`
}

func DefaultConfig() Config {
	return Config{
		Logging: clog.Config{
			Format: "text",
			Debug:  true,
		},
		Server: config.Server{
			Addr:     "127.0.0.1:3000",
			Graceful: 5,
		},

		DSN: "postgresql://postgres@localhost/dflauth?sslmode=disable",

		PrivateKey: privateKey,
		PublicKey:  publicKey,
		JWTIssuer:  "localhost:3000",

		WebAuthn: WebAuthn{
			ID:          "localhost",
			DisplayName: "DFL Auth",
			Origin:      "http://localhost:3001",
		},
	}
}

func Run(cfg Config) error {
	log := cfg.Logging.Configure()

	pgDb, err := sql.Open("postgres", cfg.DSN)
	if err != nil {
		return err
	}

	if err := pgDb.Ping(); err != nil {
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
		Logger:    log,
		WA:        web,
		SK:        sk,
		DB:        db,
		JWTIssuer: cfg.JWTIssuer,
	}

	authHandlers := auth.Handlers{
		auth.CreateScopedBearer(sk.Public(), cfg.JWTIssuer),
	}

	rpc := rpc.New(app, log, authHandlers)

	return rpc.Run(cfg.Server)
}

func wrap(a *app.App, fn func(*app.App, http.ResponseWriter, *http.Request) error) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		err := fn(a, w, r)
		if err != nil {
			rpclib.HandleError(w, r, err, a.Logger)
			return
		}
	}
}
