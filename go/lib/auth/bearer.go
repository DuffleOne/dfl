package auth

import (
	"github.com/cuvva/cuvva-public-go/lib/cher"
	"github.com/golang-jwt/jwt"
)

type Bearer struct {
	publicKey interface{}
	issuer    string
}

func CreateScopedBearer(publicKey interface{}, issuer string) Bearer {
	return Bearer{
		publicKey: publicKey,
		issuer:    issuer,
	}
}

func (b Bearer) Type() string {
	return "bearer"
}

func (b Bearer) Check(tokenStr string) (*AuthUser, error) {
	var claims DFLClaims

	_, err := jwt.ParseWithClaims(tokenStr, &claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodECDSA); !ok {
			return nil, cher.New(cher.Unauthorized, nil, cher.New("unacceptable_jwt_alg", token.Header))
		}

		return b.publicKey, nil
	})

	if err != nil {
		if jwtErr, ok := err.(*jwt.ValidationError); ok {
			if cuvvaErr, ok := jwtErr.Inner.(cher.E); ok {
				return nil, cuvvaErr
			}

			return nil, cher.New(cher.Unauthorized, nil, cher.New("jwt_parsing_failed", cher.M{"error": jwtErr}))
		}

		return nil, cher.New(cher.Unauthorized, nil, cher.New("jwt_parsing_failed", cher.M{"error": err}))
	}

	if claims.Issuer != b.issuer {
		return nil, cher.New(cher.Unauthorized, nil, cher.New("incorrect_issuer", nil))
	}

	return &AuthUser{
		ID:       claims.Subject,
		Username: claims.Username,
		Scopes:   claims.Scopes,
	}, nil
}
