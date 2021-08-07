// +build windows

package windows

import (
	"fmt"

	"github.com/cuvva/cuvva-public-go/lib/cher"
	"github.com/danieljoos/wincred"
)

const (
	prefix = "DFL"
)

type Keychain struct{}

func (k Keychain) DeleteItem(name string) error {
	cred, err := wincred.GetGenericCredential(fmt.Sprintf("%s%s", prefix, name))
	if err != nil {
		return err
	}

	return cred.Delete()
}

func (k Keychain) GetItem(name string) (data []byte, err error) {
	cred, err := wincred.GetGenericCredential(fmt.Sprintf("%s%s", prefix, name))
	if err != nil {
		if err == wincred.ErrElementNotFound {
			return nil, cher.New(cher.NotFound, nil)
		}

		return nil, err
	}

	return cred.CredentialBlob, nil
}

func (k Keychain) NewItem(name string, data []byte) error {
	cred := wincred.NewGenericCredential(fmt.Sprintf("%s%s", prefix, name))
	cred.CredentialBlob = data

	return cred.Write()
}

func (k Keychain) UpsertItem(name string, data []byte) error {
	item, err := k.GetItem(name)
	if err != nil {
		// if unknown error, or not_found
		if v, ok := err.(cher.E); !ok || v.Code != cher.NotFound {
			return err
		}
	}

	if item != nil {
		if err := k.DeleteItem(name); err != nil {
			return err
		}
	}

	return k.NewItem(name, data)
}
