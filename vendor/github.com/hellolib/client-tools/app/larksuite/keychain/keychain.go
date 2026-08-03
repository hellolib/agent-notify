package keychain

import (
	"errors"
	"fmt"

	"github.com/hellolib/client-tools/app/larksuite/output"
)

var (
	ErrNotFound       = errors.New("keychain: item not found")
	errNotInitialized = errors.New("keychain not initialized")
)

const LarkCliService = "lark-cli"

func wrapError(op string, err error) error {
	if err == nil || errors.Is(err, ErrNotFound) {
		return err
	}

	msg := fmt.Sprintf("keychain %s failed: %v", op, err)
	hint := "Check if the OS keychain/credential manager is locked or accessible. If running inside a sandbox or CI environment, please ensure the process has the necessary permissions to access the keychain."

	if errors.Is(err, errNotInitialized) {
		hint = "The keychain master key may have been cleaned up or deleted. Please reconfigure the CLI by running `lark-cli config init`."
	}

	func() {
		defer func() { recover() }()
		LogAuthError("keychain", op, fmt.Errorf("keychain %s error: %w", op, err))
	}()

	return output.ErrWithHint(output.ExitAPI, "config", msg, hint)
}

type KeychainAccess interface {
	Get(service, account string) (string, error)
	Set(service, account, value string) error
	Remove(service, account string) error
}

func Get(service, account string) (string, error) {
	val, err := platformGet(service, account)
	return val, wrapError("Get", err)
}

func Set(service, account, data string) error {
	return wrapError("Set", platformSet(service, account, data))
}

func Remove(service, account string) error {
	return wrapError("Remove", platformRemove(service, account))
}
