package app

import (
	"fmt"
	"strings"

	"github.com/cuvva/cuvva-public-go/lib/cher"
	log "github.com/sirupsen/logrus"
)

var allowedCodes = map[int]struct{}{
	200: {},
	204: {},
	302: {},
}

func (a *App) doWeb(host string, schema string, validate bool) error {
	url := fmt.Sprintf("%s://%s", schema, host)

	c := a.Client

	if !validate {
		c = a.ClientNoValidate
	}

	res, err := c.Get(url)
	if err != nil {
		if strings.Contains(err.Error(), "no such host") {
			a.Logger.Warnf("no such host, configuration error for host %s", host)
		}

		a.Logger.WithError(err).Infof("cannot connect to host %s", host)

		return cher.New("host_down", cher.M{
			"error": err,
		})
	}

	l := a.Logger.WithFields(log.Fields{
		"statusCode": res.StatusCode,
		"status":     res.Status,
	})

	if _, ok := allowedCodes[res.StatusCode]; !ok {
		l.Infof("cannot connect to host %s", host)

		return cher.New("host_bad_response", cher.M{
			"error": err,
			"res":   res,
		})
	}

	l.Infof("successfully connected to host %s", host)

	return nil
}
