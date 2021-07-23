package auth

import (
	"github.com/golang-jwt/jwt"
)

type DFLClaims struct {
	Version  string `json:"v"`
	Scopes   string `json:"scopes"`
	Username string `json:"username"`
	jwt.StandardClaims
}
