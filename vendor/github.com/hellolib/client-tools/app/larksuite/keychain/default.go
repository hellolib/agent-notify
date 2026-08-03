package keychain

type defaultKeychain struct{}

func (d *defaultKeychain) Get(service, account string) (string, error) {
	return Get(service, account)
}

func (d *defaultKeychain) Set(service, account, value string) error {
	return Set(service, account, value)
}

func (d *defaultKeychain) Remove(service, account string) error {
	return Remove(service, account)
}

func Default() KeychainAccess {
	return &defaultKeychain{}
}
