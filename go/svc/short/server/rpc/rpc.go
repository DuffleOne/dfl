package rpc

import (
	"net/http"

	"dfl/lib/auth"
	"dfl/svc/short/server/app"

	"github.com/cuvva/cuvva-public-go/lib/config"
	"github.com/cuvva/cuvva-public-go/lib/crpc"
	"github.com/cuvva/cuvva-public-go/lib/middleware/request"
	"github.com/cuvva/cuvva-public-go/lib/version"
	"github.com/go-chi/chi"
	"github.com/rs/cors"
	"github.com/sirupsen/logrus"
)

type RPC struct {
	app *app.App
	log *logrus.Entry

	httpServer *http.Server
}

func New(app *app.App, log *logrus.Entry, authHandlers auth.Auth, htmlPages, vanilla *chi.Mux) *RPC {
	rpc := &RPC{
		app: app,
		log: log,
	}

	zs := crpc.NewServer(auth.AllowAllAuthenticated)
	zs.Use(crpc.Logger())

	zs.Register("add_shortcut", "2021-07-17", addShortcutSchema, rpc.AddShortcut)
	zs.Register("create_signed_url", "2021-07-17", createSignedURLSchema, rpc.CreateSignedURL)
	zs.Register("delete_resource", "2021-07-17", deleteResourceSchema, rpc.DeleteResource)
	zs.Register("list_resources", "2021-07-17", listResourcesSchema, rpc.ListResources)
	zs.Register("remove_shortcut", "2021-07-17", removeShortcutSchema, rpc.RemoveShortcut)
	zs.Register("resave_hashes", "2021-07-17", nil, rpc.ResaveHashes)
	zs.Register("set_nsfw", "2021-07-17", setNSFWSchema, rpc.SetNSFW)
	zs.Register("shorten_url", "2021-07-17", shortenURLSchema, rpc.ShortenURL)
	zs.Register("view_details", "2021-07-17", viewDetailsSchema, rpc.ViewDetails)

	mux := chi.NewRouter()

	mux.Use(version.Header("service-short"))
	mux.Use(request.StripPrefix("/1/short"))

	mux.
		With(
			request.RequestID,
			request.Logger(log),
			cors.AllowAll().Handler,
			auth.Middleware(authHandlers),
			request.StripPrefix("/1"),
		).
		Handle("/1/*", zs)

	mux.With(request.StripPrefix("/0")).Handle("/0/*", vanilla)

	mux.Handle("/*", htmlPages)

	rpc.httpServer = &http.Server{Handler: mux}

	return rpc
}

// Run the RPC server and listen on the specified address
func (r *RPC) Run(cfg config.Server) error {
	r.log.WithField("addr", cfg.Addr).Info("listening")

	if err := cfg.ListenAndServe(r.httpServer); err != nil {
		return err
	}

	return nil
}
