package pipeline

import (
	"dfl/lib/rpc"
)

// HandleNSFWPrimer will show a NSFW primer screen if a resource has NSFW content
func HandleNSFWPrimer(p *Pipeline) (bool, error) {
	if p.context.forceDownload || p.context.primed {
		return true, nil
	}

	var anyNSFW bool

	for _, i := range p.rwqs {
		rwq := i

		if rwq.r.NSFW {
			anyNSFW = true
		}
	}

	if !anyNSFW {
		return true, nil
	}

	return false, rpc.QuickTemplate(p.w, nil, []string{
		"./resources/short/nsfw.html",
		"./resources/short/layouts/root.html",
	})
}
