package keychain

type Keychain interface {
	DeleteItem(name string) error
	GetItem(name string) (data []byte, err error)
	NewItem(name string, data []byte) error
	UpsertItem(name string, data []byte) error
}
