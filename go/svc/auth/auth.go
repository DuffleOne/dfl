package auth

import (
	"context"
	"time"

	"github.com/duo-labs/webauthn/webauthn"
)

type Service interface {
	CreateClient(context.Context, *CreateClientRequest) (*CreateClientResponse, error)
	CreateInviteCode(context.Context, *CreateInviteCodeRequest) (*CreateInviteCodeResponse, error)
	Token(context.Context, *TokenRequest) (*TokenResponse, error)
	WhoAmI(context.Context) (*WhoAmIResponse, error)

	WithoutAuth() Service
}

type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Email     *string   `json:"email"`
	Scopes    string    `json:"scopes"`
	CreatedAt time.Time `json:"created_at"`
}

type Invitation struct {
	ID         string     `json:"id"`
	Code       string     `json:"code"`
	Scopes     string     `json:"scopes"`
	CreatedBy  *string    `json:"created_by"`
	CreatedAt  time.Time  `json:"created_at"`
	RedeemedBy *string    `json:"redeemed_by"`
	RedeemedAt *time.Time `json:"redeemed_at"`
	ExpiresAt  *time.Time `json:"expires_at"`
}

type Client struct {
	ID           string    `json:"client_id"`
	Name         string    `json:"name"`
	RedirectURIs []string  `json:"redirect_uris"`
	CreatedAt    time.Time `json:"created_at"`
}

type AuthorizationCode struct {
	ID                  string    `json:"id"`
	ClientID            string    `json:"client_id"`
	ResponseType        string    `json:"response_type"`
	RedirectURI         *string   `json:"redirect_uri"`
	State               string    `json:"state"`
	Scope               string    `json:"scope"`
	Nonce               string    `json:"nonce"`
	CodeChallengeMethod string    `json:"code_challenge_method"`
	CodeChallenge       string    `json:"code_challenge"`
	UserID              string    `json:"user_id"`
	CreatedAt           time.Time `json:"created_at"`
	ExpiresAt           time.Time `json:"expires_at"`
}

type WebAuthnRegister struct {
	ID       string `json:"id"`
	RawID    string `json:"raw_id"`
	Type     string `json:"type"`
	Response struct {
		AttestationObject string `json:"attestation_object"`
		ClientDataJSON    string `json:"client_data_json"`
	} `json:"response"`
}

type WebAuthnLogin struct {
	ID       string `json:"id"`
	RawID    string `json:"raw_id"`
	Type     string `json:"type"`
	Response struct {
		AuthenticatorData string `json:"authenticator_data"`
		ClientDataJSON    string `json:"client_data_json"`
		Signature         string `json:"signature"`
		UserHandle        string `json:"user_handle"`
	} `json:"response"`
}

type U2FCredential struct {
	ID        string
	UserID    string
	Name      *string
	SignedAt  *time.Time
	DeletedAt *time.Time
	webauthn.Credential
}

type RegisterPromptRequest struct {
	Username   string `json:"username"`
	InviteCode string `json:"invite_code"`
}

type RegisterPromptResponse struct {
	ID        string      `json:"id"`
	Challenge interface{} `json:"challenge"`
}

type RegisterConfirmRequest struct {
	Username    string           `json:"username"`
	InviteCode  string           `json:"invite_code"`
	KeyName     *string          `json:"key_name"`
	ChallengeID string           `json:"challenge_id"`
	WebAuthn    WebAuthnRegister `json:"webauthn"`
}

type AuthorizePromptRequest struct {
	Username string `json:"username"`
}

type AuthorizePromptResponse struct {
	ID        string      `json:"id"`
	Challenge interface{} `json:"challenge"`
}

type AuthorizeConfirmRequest struct {
	ClientID            string        `json:"client_id"`
	CodeChallenge       string        `json:"code_challenge"`
	CodeChallengeMethod string        `json:"code_challenge_method"`
	Nonce               string        `json:"nonce"`
	RedirectURI         *string       `json:"redirect_uri"`
	ResponseType        string        `json:"response_type"`
	Scope               string        `json:"scope"`
	State               string        `json:"state"`
	Username            string        `json:"username"`
	ChallengeID         string        `json:"challenge_id"`
	WebAuthn            WebAuthnLogin `json:"webauthn"`
}

type AuthorizeConfirmResponse struct {
	Type   string      `json:"type"`
	Params interface{} `json:"params"`
}

type AuthorizeWithRedirectParams struct {
	URI string `json:"uri"`
}

type AuthorizeWithoutRedirectParams struct {
	AuthorizationCode string `json:"authorization_code"`
	ExpiresAt         string `json:"expires_at"`
	ExpiresIn         int    `json:"expires_in"`
	State             string `json:"state"`
}

type WhoAmIResponse struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
}

type CreateClientRequest struct {
	Name         string   `json:"name"`
	RedirectURIs []string `json:"redirect_uris"`
}

type CreateClientResponse struct {
	ClientID string `json:"client_id"`
}

type TokenRequest struct {
	ClientID     string  `json:"client_id"`
	GrantType    string  `json:"grant_type"`
	RedirectURI  *string `json:"redirect_uri"`
	Code         string  `json:"code"`
	CodeVerifier string  `json:"code_verifier"`
}

type TokenResponse struct {
	UserID      string `json:"user_id"`
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Expires     int    `json:"expires"`
}

type GetClientRequest struct {
	ClientID string `json:"client_id"`
}

type GetClientResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ListU2FKeysRequest struct {
	UserID          string `json:"user_id"`
	IncludeUnsigned bool   `json:"include_unsigned"`
}

type PublicU2FKey struct {
	ID        string     `json:"id"`
	Name      *string    `json:"name"`
	SignedAt  *time.Time `json:"signed_at"`
	PublicKey string     `json:"public_key"`
}

type CreateKeyPromptRequest struct {
	UserID string `json:"user_id"`
}

type CreateKeyPromptResponse struct {
	ID        string      `json:"id"`
	Challenge interface{} `json:"challenge"`
}

type CreateKeyConfirmRequest struct {
	UserID      string           `json:"user_id"`
	KeyName     *string          `json:"key_name"`
	ChallengeID string           `json:"challenge_id"`
	WebAuthn    WebAuthnRegister `json:"webauthn"`
}

type DeleteKeyRequest struct {
	UserID string `json:"user_id"`
	KeyID  string `json:"key_id"`
}

type SignKeyPromptRequest struct {
	UserID    string `json:"user_id"`
	KeyToSign string `json:"key_to_sign"`
}

type SignKeyPromptResponse struct {
	ID        string      `json:"id"`
	Challenge interface{} `json:"challenge"`
}

type SignKeyConfirmRequest struct {
	UserID      string        `json:"user_id"`
	KeyToSign   string        `json:"key_to_sign"`
	ChallengeID string        `json:"challenge_id"`
	WebAuthn    WebAuthnLogin `json:"webauthn"`
}

type CreateInviteCodeRequest struct {
	Code      *string    `json:"code"`
	ExpiresAt *time.Time `json:"expires_at"`
	Scopes    string     `json:"scopes"`
}

type CreateInviteCodeResponse struct {
	Code      string     `json:"code"`
	ExpiresAt *time.Time `json:"expires_at"`
}
