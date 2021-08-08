// +build darwin

package darwin

import (
	"fmt"
	"os/user"

	"github.com/cuvva/cuvva-public-go/lib/cher"
	gokeychain "github.com/keybase/go-keychain"
)

const (
	service     = "mn.dfl.auth"
	accessGroup = "mn.dfl.auth"
	prefix      = "DFL"
)

type Keychain struct{}

func (k Keychain) createQueryItem(name string, read bool) (*gokeychain.Item, error) {
	user, err := user.Current()
	if err != nil {
		return nil, err
	}

	item := gokeychain.NewItem()
	item.SetSecClass(gokeychain.SecClassGenericPassword)
	item.SetService(service)
	item.SetLabel(fmt.Sprintf("%s %s", prefix, name))
	item.SetAccessGroup(accessGroup)
	item.SetAccount(user.Username)
	item.SetSynchronizable(gokeychain.SynchronizableAny)
	item.SetAccessible(gokeychain.AccessibleAlways)

	if read {
		item.SetReturnData(true)
		item.SetReturnAttributes(true)
	}

	return &item, nil
}

func (k Keychain) NewItem(name string, data []byte) error {
	item, err := k.createQueryItem(name, false)
	if err != nil {
		return err
	}

	return k.newItem(item, data)
}

func (k Keychain) newItem(item *gokeychain.Item, data []byte) error {
	item.SetData(data)

	if err := gokeychain.AddItem(*item); err != nil {
		return err
	}

	return nil
}

func (k Keychain) UpsertItem(name string, data []byte) error {
	item, err := k.createQueryItem(name, false)
	if err != nil {
		return err
	}

	var existingItem *gokeychain.QueryResult

	existingItem, err = k.getItem(name)
	if v, ok := err.(cher.E); err != nil && (!ok || v.Code != cher.NotFound) {
		return err
	}

	if existingItem == nil {
		return k.newItem(item, data)
	}

	newItem, err := k.createQueryItem(name, false)
	if err != nil {
		return err
	}

	newItem.SetData(data)

	return gokeychain.UpdateItem(*item, *newItem)
}

func (k Keychain) getItem(name string) (*gokeychain.QueryResult, error) {
	query, err := k.createQueryItem(name, true)
	if err != nil {
		return nil, err
	}

	results, err := gokeychain.QueryItem(*query)
	if err != nil || len(results) == 0 {
		if err == gokeychain.ErrorItemNotFound || len(results) == 0 {
			if err == nil {
				return nil, cher.New(cher.NotFound, nil)
			}

			return nil, cher.New(cher.NotFound, nil, cher.Coerce(err))
		}

		return nil, err
	}

	if len(results) > 1 {
		return nil, cher.New("multiple_results", nil)
	}

	return &results[0], nil
}

func (k Keychain) GetItem(name string) (data []byte, err error) {
	item, err := k.getItem(name)
	if err != nil {
		return nil, err
	}

	return item.Data, nil
}

func (k Keychain) DeleteItem(name string) error {
	item, err := k.createQueryItem(name, false)
	if err != nil {
		return err
	}

	err = gokeychain.DeleteItem(*item)
	if err == gokeychain.ErrorItemNotFound {
		return cher.New(cher.NotFound, nil)
	}

	return err
}
