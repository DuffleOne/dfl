package cli

import (
	"encoding/json"

	"dfl/lib/keychain"
	"dfl/svc/auth"

	"github.com/cuvva/cuvva-public-go/lib/cher"
	"github.com/cuvva/cuvva-public-go/lib/ptr"
)

func AuthHeader(keychain keychain.Keychain, tool string) (*string, error) {
	var authBytes []byte
	var err error

	if keychain == nil {
		return nil, nil
	}

	authBytes, err = keychain.GetItem("Auth")
	if err != nil {
		if v, ok := err.(cher.E); ok && v.Code == cher.NotFound {
			return nil, nil
		}

		return nil, err
	}

	res := auth.TokenResponse{}

	err = json.Unmarshal(authBytes, &res)
	if err != nil {
		return nil, err
	}

	return ptr.String(res.AccessToken), nil
}
