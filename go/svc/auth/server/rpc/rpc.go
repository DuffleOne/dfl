package rpc

import (
	"net/http"

	"dfl/lib/auth"
	authSvc "dfl/svc/auth"
	"dfl/svc/auth/server/app"

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

func New(app *app.App, log *logrus.Entry, authHandlers auth.Auth) *RPC {
	rpc := &RPC{
		app: app,
		log: log,
	}

	zs := crpc.NewServer(auth.UnsafeNoAuthentication)
	zs.Use(crpc.Logger())

	zs.Register("authorize_confirm", "2021-01-15", authorizeConfirmSchema, rpc.AuthorizeConfirm)
	zs.Register("authorize_prompt", "2021-01-15", authorizePromptSchema, rpc.AuthorizePrompt)
	zs.Register("register_confirm", "2021-01-15", registerConfirmSchema, rpc.RegisterConfirm)
	zs.Register("register_prompt", "2021-01-15", registerPromptSchema, rpc.RegisterPrompt)
	zs.Register("token", "2021-01-15", tokenSchema, rpc.Token)
	zs.Register("get_client", "2021-01-15", getClientSchema, rpc.GetClient)

	zs.Use(auth.AllowAllAuthenticated)
	zs.Register("create_client", "2021-01-15", createClientSchema, rpc.CreateClient)
	zs.Register("create_invite_code", "2021-01-15", createInviteCodeSchema, rpc.CreateInviteCode)
	zs.Register("create_key_confirm", "2021-01-15", createKeyConfirmSchema, rpc.CreateKeyConfirm)
	zs.Register("create_key_prompt", "2021-01-15", createKeyPromptSchema, rpc.CreateKeyPrompt)
	zs.Register("delete_key", "2021-01-15", deleteKeySchema, rpc.DeleteKey)
	zs.Register("list_u2f_keys", "2021-01-15", listU2FKeysSchema, rpc.ListU2FKeys)
	zs.Register("sign_key_confirm", "2021-01-15", signKeyConfirmSchema, rpc.SignKeyConfirm)
	zs.Register("sign_key_prompt", "2021-01-15", signKeyPromptSchema, rpc.SignKeyPrompt)
	zs.Register("whoami", "2021-01-15", nil, rpc.WhoAmI)

	mux := chi.NewRouter()

	mux.Use(version.Header("service-auth"))
	mux.Use(request.StripPrefix("/1/auth"))

	mux.Get("/system/health", request.HealthCheck)

	mux.
		With(
			request.RequestID,
			request.Logger(log),
			cors.AllowAll().Handler,
			auth.Middleware(authHandlers),
			request.StripPrefix("/1"),
		).
		Handle("/1/*", zs)

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

func (r *RPC) WithoutAuth() authSvc.Service {
	return nil
}
