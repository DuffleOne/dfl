package server

import (
	"database/sql"
	"net/http"
	"os"

	"dfl/lib/auth"
	"dfl/lib/cache"
	"dfl/lib/key"
	rpclib "dfl/lib/rpc"
	"dfl/svc/short/server/app"
	"dfl/svc/short/server/db"
	"dfl/svc/short/server/html"
	"dfl/svc/short/server/lib/storageproviders"
	"dfl/svc/short/server/rpc"
	"dfl/svc/short/server/vanilla"

	"github.com/cuvva/cuvva-public-go/lib/cher"
	"github.com/cuvva/cuvva-public-go/lib/clog"
	"github.com/cuvva/cuvva-public-go/lib/config"
	"github.com/go-chi/chi"
	"github.com/go-redis/redis"
	_ "github.com/lib/pq" // required for the PGSQL driver to be loaded
	"github.com/nishanths/go-xkcd/v2"
	"github.com/speps/go-hashids"
)

const publicKey = `-----BEGIN PUBLIC KEY-----
MHYwEAYHKoZIzj0CAQYFK4EEACIDYgAEMc43MCK/BZKT+tfeOre/tAhNzi2OvrEy
ivDKI4ixaJhKSdjGKLvkoz3O5tGUgPWItugWuUMohzYDqOnDVUAaza2HeO3SUOGG
KeKIzFcp4NSDmjdLUjt+8a6/wY3EAqxM
-----END PUBLIC KEY-----`

type Config struct {
	Logging clog.Config   `envconfig:"logging"`
	Server  config.Server `envconfig:"server"`

	DSN string `envconfig:"dsn"`

	PublicKey string `envconfig:"public_key"`
	JWTIssuer string `envconfig:"jwt_issuer"`

	Salt    string `envconfig:"salt"`
	RootURL string `envconfig:"root_url"`

	StorageProvider string `envconfig:"storage_provider"`

	LFSFolder      string      `envconfig:"lfs_folder"`
	LFSPermissions os.FileMode `envconfig:"lfs_permissions"`

	AWSRegion string `envconfig:"aws_region"`
	AWSBucket string `envconfig:"aws_bucket"`
	AWSRoot   string `envconfig:"aws_root"`

	RedisURI string `envconfig:"redis_uri"`
}

func DefaultConfig() Config {
	return Config{
		Logging: clog.Config{
			Format: "text",
			Debug:  true,
		},
		Server: config.Server{
			Addr:     "127.0.0.1:3001",
			Graceful: 5,
		},

		DSN: "postgresql://postgres@localhost/dflimg?sslmode=disable",

		PublicKey: publicKey,
		JWTIssuer: "localhost:3000",

		Salt:    "savour-shingle-sidney-rajah-punk-lead-jenny-scot",
		RootURL: "http://localhost:3001",

		StorageProvider: "lfs",

		LFSFolder:      "/Users/duffleman/Downloads/short",
		LFSPermissions: 0644,

		AWSRegion: "eu-west-1",
		AWSBucket: "s3.duffleman.co.uk",
		AWSRoot:   "i.dfl.mn",

		RedisURI: "redis://localhost:6379",
	}
}

func Run(cfg Config) error {
	log := cfg.Logging.Configure()

	var err error
	var sp storageproviders.StorageProvider

	switch cfg.StorageProvider {
	case "aws":
		sp, err = storageproviders.NewAWSProviderFromCfg(cfg.AWSRegion, cfg.AWSBucket, cfg.AWSRoot)
		if err != nil {
			return err
		}
	case "lfs":
		sp, err = storageproviders.NewLFSProviderFromCfg(cfg.LFSFolder, cfg.LFSPermissions)
		if err != nil {
			return err
		}
	default:
		return cher.New("unsupported_provider", nil)
	}

	pgDb, err := sql.Open("postgres", cfg.DSN)
	if err != nil {
		return err
	}

	if err := pgDb.Ping(); err != nil {
		return err
	}

	public := key.MustParseECDSAPublic([]byte(cfg.PublicKey))

	db := db.New(pgDb)

	hd := hashids.NewData()
	hd.Salt = cfg.Salt
	hd.MinLength = 3

	hasher, err := hashids.NewWithData(hd)
	if err != nil {
		return err
	}

	redisOpts, err := redis.ParseURL(cfg.RedisURI)
	if err != nil {
		return err
	}

	redisClient := redis.NewClient(redisOpts)
	_, err = redisClient.Ping().Result()
	if err != nil {
		return err
	}

	redis := cache.NewCache(redisClient)

	xkcdClient := xkcd.NewClient()

	app := &app.App{
		Logger: log,

		SP:     sp,
		DB:     db,
		Hasher: hasher,
		Redis:  redis,

		XKCD: xkcdClient,

		RootURL: cfg.RootURL,
	}

	authHandlers := auth.Handlers{
		auth.CreateScopedBearer(public, cfg.JWTIssuer),
	}

	html := htmlPages(app)
	vanilla := vanillaFuncs(app, authHandlers)

	rpc := rpc.New(app, log, authHandlers, html, vanilla)

	return rpc.Run(cfg.Server)
}

func vanillaFuncs(app *app.App, authHandlers auth.Auth) *chi.Mux {
	mux := chi.NewRouter()

	mux.Use(auth.Middleware(authHandlers))
	mux.Post("/upload_file", wrap(app, vanilla.UploadFile))

	return mux
}

func htmlPages(app *app.App) *chi.Mux {
	mux := chi.NewRouter()

	mux.Get("/", wrap(app, html.Index))
	mux.Get("/robots.txt", wrap(app, html.Robots))
	mux.Get("/{query}", wrap(app, html.HandleResource))
	mux.Head("/{query}", wrap(app, html.HeadResource))

	return mux
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
