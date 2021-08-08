package app

import (
	"fmt"

	"dfl/lib/cli"
	"dfl/lib/keychain"
	"dfl/svc/auth"
	"dfl/svc/short"
)

type App struct {
	APIURL     string
	UIURL      string
	AuthAPIURL string
	AuthUIURL  string
	Keychain   keychain.Keychain
	Client     short.Service
	AuthClient auth.Service
}

func New(apiURL, uiURL, authAPIURL, authUIURL string, kc keychain.Keychain) (*App, error) {
	bearerToken, err := cli.AuthHeader(kc, "short")
	if err != nil {
		return nil, err
	}

	client, err := short.NewClient(fmt.Sprintf("%s/", apiURL), bearerToken), nil
	if err != nil {
		return nil, err
	}

	authClient, err := auth.NewClient(fmt.Sprintf("%s/", authAPIURL), bearerToken), nil
	if err != nil {
		return nil, err
	}

	return &App{
		APIURL:     apiURL,
		UIURL:      uiURL,
		AuthAPIURL: authAPIURL,
		AuthUIURL:  authUIURL,
		Keychain:   kc,
		Client:     client,
		AuthClient: authClient,
	}, nil
}

func (a *App) GetAuthClient() auth.Service {
	return a.AuthClient
}

func (a *App) GetKeychain() keychain.Keychain {
	return a.Keychain
}

func (a *App) GetAPIURL() string {
	return a.AuthAPIURL
}

func (a *App) GetUIURL() string {
	return a.AuthUIURL
}
