package app

import (
	"fmt"

	"dfl/lib/cli"
	"dfl/lib/keychain"
	"dfl/svc/auth"
)

type App struct {
	RootURL  string
	Keychain keychain.Keychain
	Client   auth.Service
}

func New(rootURL string, kc keychain.Keychain) (*App, error) {
	bearerToken, err := cli.AuthHeader(kc, "auth")
	if err != nil {
		return nil, err
	}

	client, err := auth.NewClient(fmt.Sprintf("%s/", rootURL), bearerToken), nil
	if err != nil {
		return nil, err
	}

	return &App{
		RootURL:  rootURL,
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

func (a *App) GetAuthURL() string {
	return a.RootURL
}
