package version_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	dflhttp "github.com/duffleone/dfl/http"
	"github.com/duffleone/dfl/http/version"
)

type greetingV1 struct {
	Greeting string `json:"greeting"`
}

type greetingV2 struct {
	Greeting string `json:"greeting"`
	Language string `json:"language"`
}

// Example shows the whole loop: one resolver for the API, an endpoint with
// a variant per date, and Stripe-style pinning deciding which one answers.
func Example() {
	api := version.NewResolver(version.Dates(),
		version.Header("X-API-Version"),
	)

	greet := version.NewEndpoint(api)
	version.Handle(greet, "2024-01-02", func(context.Context, *dflhttp.Empty) (*greetingV1, error) {
		return &greetingV1{Greeting: "hello"}, nil
	})
	version.Handle(greet, "2024-06-01", func(context.Context, *dflhttp.Empty) (*greetingV2, error) {
		return &greetingV2{Greeting: "hello", Language: "en"}, nil
	})

	r := dflhttp.NewRouter(http.NewServeMux())
	r.HandleFunc(http.MethodGet, "/greeting", greet.Serve)

	for _, pin := range []string{"2024-01-02", "2024-03-15", "2024-06-01"} {
		req := httptest.NewRequest(http.MethodGet, "/greeting", nil)
		req.Header.Set("X-API-Version", pin)

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		fmt.Printf("%s -> %s", pin, rec.Body.String())
	}

	// Output:
	// 2024-01-02 -> {"greeting":"hello"}
	// 2024-03-15 -> {"greeting":"hello"}
	// 2024-06-01 -> {"greeting":"hello","language":"en"}
}
