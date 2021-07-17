package app

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"path"

	"dfl/lib/key"
	"dfl/tools/certgen"

	"github.com/pterm/pterm"
	"github.com/tjarratt/babble"
)

func (a *App) VerifyKeyPair(name string) error {
	if err := a.checkForInit(); err != nil {
		return err
	}

	privFilePath := path.Join(
		a.RootDirectory,
		certgen.CertFolderMap[certgen.KeyPair],
		fmt.Sprintf("%s.private.pem", name),
	)

	pubFilePath := path.Join(
		a.RootDirectory,
		certgen.CertFolderMap[certgen.KeyPair],
		fmt.Sprintf("%s.public.pem", name),
	)

	privatePemBytes, err := ioutil.ReadFile(privFilePath)
	if err != nil {
		return err
	}

	publicPemBytes, err := ioutil.ReadFile(pubFilePath)
	if err != nil {
		return err
	}

	private, err := key.ParseECDSAPrivate(privatePemBytes)
	if err != nil {
		return err
	}

	public, err := key.ParseECDSAPublic(publicPemBytes)
	if err != nil {
		return err
	}

	message := makeMessage()
	hash := sha256.Sum256([]byte(message))

	signature, err := ecdsa.SignASN1(rand.Reader, private, hash[:])
	if err != nil {
		return err
	}

	valid := ecdsa.VerifyASN1(public, hash[:], signature)

	fmt.Printf("Message: %s\n", message)
	fmt.Print("Signature ")

	switch valid {
	case true:
		pterm.BgGreen.Println("VERIFIED")
	default:
		pterm.BgRed.Println("MISMATCH")
	}

	return nil
}

func makeMessage() string {
	babbler := babble.NewBabbler()
	babbler.Count = 4

	return babbler.Babble()
}
