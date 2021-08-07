package key

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"

	"github.com/cuvva/cuvva-public-go/lib/cher"
)

func MustParseECDSAPublic(inBytes []byte) *ecdsa.PublicKey {
	cert, err := ParseECDSAPublic(inBytes)
	if err != nil {
		panic(err)
	}

	return cert
}

func ParseECDSAPublic(inBytes []byte) (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode(inBytes)
	x509Encoded := block.Bytes

	cert, err := x509.ParsePKIXPublicKey(x509Encoded)
	if err != nil {
		return nil, err
	}

	if v, ok := cert.(*ecdsa.PublicKey); ok {
		return v, nil
	}

	return nil, cher.New("cannot_parse_cert", nil)
}

func MustParseECDSAPrivate(inBytes []byte) *ecdsa.PrivateKey {
	cert, err := ParseECDSAPrivate(inBytes)
	if err != nil {
		panic(err)
	}

	return cert
}

func ParseECDSAPrivate(inBytes []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(inBytes)
	x509Encoded := block.Bytes

	return x509.ParseECPrivateKey(x509Encoded)
}
