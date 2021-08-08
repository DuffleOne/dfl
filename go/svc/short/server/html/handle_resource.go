package html

import (
	"net/http"

	"dfl/svc/short/server/app"
	"dfl/svc/short/server/html/pipeline"

	"github.com/go-chi/chi"
)

func HandleResource(a *app.App, w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	qi := a.ParseQueryType(chi.URLParam(r, "query"))

	pipe := pipeline.New(ctx, &pipeline.Creator{
		App: a,
		R:   r,
		W:   w,
		QI:  qi,
	})

	pipe.Steps([]pipeline.HandlerType{
		// ordered
		pipeline.ListFromDB,
		pipeline.HandleDeleted,
		pipeline.MakeContext,
		pipeline.ValidateRequest,
		pipeline.HandleNSFWPrimer,
		pipeline.HandleURLRedirect,
		pipeline.LoadFilesFromFS,
		pipeline.SyntaxHighlighter,
		pipeline.RenderMD,
		pipeline.MakeTarball,
		pipeline.FilterMultiFile,
		pipeline.ServeContent,
	})

	return pipe.Run()
}
