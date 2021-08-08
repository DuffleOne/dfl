package app

import (
	"fmt"

	"dfl/lib/cli"
	"dfl/lib/keychain"
	"dfl/svc/auth"
)

type App struct {
	APIURL   string
	UIURL    string
	Keychain keychain.Keychain
	Client   auth.Service
}

func New(APIURL, UIURL string, kc keychain.Keychain) (*App, error) {
	bearerToken, err := cli.AuthHeader(kc, "auth")
	if err != nil {
		return nil, err
	}

	client, err := auth.NewClient(fmt.Sprintf("%s/", APIURL), bearerToken), nil
	if err != nil {
		return nil, err
	}

	return &App{
		APIURL:   APIURL,
		UIURL:    UIURL,
		Keychain: kc,
		Client:   client,
	}, nil
}

func (a *App) GetAuthClient() auth.Service {
	return a.Client
}

func (a *App) GetKeychain() keychain.Keychain {
	return a.Keychain
}

func (a *App) GetAPIURL() string {
	return a.APIURL
}

func (a *App) GetUIURL() string {
	return a.UIURL
}
