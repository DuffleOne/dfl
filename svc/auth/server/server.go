package server

import (
	"database/sql"
	"fmt"
	"net/http"

	dflmw "dfl/lib/middleware"
	"dfl/lib/ptr"
	"dfl/svc/auth/server/app"
	"dfl/svc/auth/server/db"
	"dfl/svc/auth/server/rpc"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	_ "github.com/lib/pq" // required for the PGSQL driver to be loaded
	"github.com/rs/cors"
	"github.com/sirupsen/logrus"
)

const privateKey = `-----BEGIN EC PRIVATE KEY-----
MIG2AgEAMBAGByqGSM49AgEGBSuBBAAiBIGeMIGbAgEBBDDefgldBHt1KEe8MgAp
OYnvcasrtBarz6T1+8BZqmrMjMS3qb8Hdhpkrp+9FKDUE2GhZANiAARUJoVEBFGG
D83t4E6QbcvGydmDbak9Jr9Osyy9Q9zj1vOopOXjFYla5DsVHTdGV5sHHnEEjEsT
kF4uOqSaFqrtqvnmhFyrjhflf4zAouCm2xK8kX7ueRizVw1E69AUAhs=
-----END EC PRIVATE KEY-----`

const publicKey = `-----BEGIN PUBLIC KEY-----
MHYwEAYHKoZIzj0CAQYFK4EEACIDYgAEVCaFRARRhg/N7eBOkG3LxsnZg22pPSa/
TrMsvUPc49bzqKTl4xWJWuQ7FR03RlebBx5xBIxLE5BeLjqkmhaq7ar55oRcq44X
5X+MwKLgptsSvJF+7nkYs1cNROvQFAIb
-----END PUBLIC KEY-----`

type Config struct {
	Logger *logrus.Logger

	Port int    `envconfig:"port"`
	DSN  string `envconfig:"dsn"`

	PrivateKey string `envconfig:"private_key"`
	PublicKey  string `envconfig:"public_key"`
}

func DefaultConfig() Config {
	return Config{
		Logger: logrus.New(),
		Port:   3000,

		DSN: "postgresql://postgres@localhost/dflauth?sslmode=disable",

		PrivateKey: privateKey,
		PublicKey:  publicKey,
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

	sk, err := app.ParseKeys(&app.SigningKeysInput{
		Public:  cfg.PublicKey,
		Private: cfg.PrivateKey,
	})
	if err != nil {
		return err
	}

	app := &app.App{
		SK: sk,
		DB: db,
	}

	router := chi.NewRouter()

	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)
	router.Use(cors.AllowAll().Handler)
	router.Use(dflmw.AuthMiddleware(sk.Public(), []dflmw.HTTPResource{
		{Verb: ptr.String(http.MethodGet), Path: ptr.String("/authorize")},
		{Verb: ptr.String(http.MethodGet), Path: ptr.String("/get_public_cert")},
		{Verb: ptr.String(http.MethodPost), Path: ptr.String("/authorize")},
		{Verb: ptr.String(http.MethodPost), Path: ptr.String("/login")},
		{Verb: ptr.String(http.MethodPost), Path: ptr.String("/token")},
	}))

	router.Get("/get_public_cert", rpc.GetPublicCert(app))
	router.Get("/authorize", rpc.AuthorizeGet(app))
	router.Post("/authorize", rpc.AuthorizePost(app))
	router.Post("/create_client", rpc.CreateClient(app))
	router.Post("/register", rpc.Register(app))
	router.Post("/token", rpc.Token(app))
	router.Post("/whoami", rpc.WhoAmI(app))

	cfg.Logger.Infof("Server running on port %d", cfg.Port)
	return http.ListenAndServe(fmt.Sprintf(":%d", cfg.Port), router)
}