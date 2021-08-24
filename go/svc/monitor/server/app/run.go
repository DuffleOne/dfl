package app

import (
	"context"
	"time"

	"github.com/alexliesenfeld/health"
	"github.com/alexliesenfeld/health/interceptors"
	sdk "github.com/andygrunwald/cachet"
)

func (a *App) Run(ctx context.Context) health.Checker {
	return health.NewChecker(
		health.WithCacheDuration(1*time.Second),
		health.WithTimeout(10*time.Second),
		health.WithInterceptors(
			interceptors.BasicLogger(),
			cachetInterceptor(a),
		),

		health.WithPeriodicCheck(15*time.Second, 1*time.Second, health.Check{
			Name: "plex",
			Check: func(ctx context.Context) error {
				host, scheme, validate := a.Get("plex", "plex.lauraflix.uk:32400/web/index.html", "https", false)

				return a.doWeb(host, scheme, validate)
			},
		}),

		health.WithPeriodicCheck(15*time.Second, 1*time.Second, health.Check{
			Name: "overseerr",
			Check: func(ctx context.Context) error {
				host, scheme, validate := a.Get("overseerr", "requests.lauraflix.uk", "https", true)

				return a.doWeb(host, scheme, validate)
			},
		}),

		health.WithPeriodicCheck(15*time.Second, 2*time.Second, health.Check{
			Name: "synclounge",
			Check: func(ctx context.Context) error {
				host, scheme, validate := a.Get("synclounge", "sync.lauraflix.uk", "https", true)

				return a.doWeb(host, scheme, validate)
			},
		}),

		health.WithPeriodicCheck(15*time.Second, 3*time.Second, health.Check{
			Name: "dfl-auth",
			Check: func(ctx context.Context) error {
				host, scheme, validate := a.Get("dfl-auth", "api.duffle.one/1/auth/system/health", "https", true)

				return a.doWeb(host, scheme, validate)
			},
		}),

		health.WithPeriodicCheck(15*time.Second, 4*time.Second, health.Check{
			Name: "dfl-short",
			Check: func(ctx context.Context) error {
				host, scheme, validate := a.Get("dfl-short", "api.duffle.one/1/short/system/health", "https", true)

				return a.doWeb(host, scheme, validate)
			},
		}),
	)
}

func cachetInterceptor(a *App) health.Interceptor {
	return func(next health.InterceptorFunc) health.InterceptorFunc {
		return func(ctx context.Context, name string, state health.CheckState) health.CheckState {
			result := next(ctx, name, state)

			componentName, ok := a.CachetNames[name]
			if !ok {
				return result
			}

			componentID, ok := a.Cache.m[componentName]
			if !ok {
				return result
			}

			newStatus := sdk.ComponentStatusMajorOutage

			if result.Status == "up" {
				newStatus = sdk.ComponentStatusOperational
			}

			a.Cachet.Components.Update(componentID, &sdk.Component{
				Status: newStatus,
			})

			return result
		}
	}
}
