package cmdutil

import (
	"io"

	"github.com/hellolib/client-tools/app/larksuite/keychain"
)

type IOStreams struct {
	In         io.Reader
	Out        io.Writer
	ErrOut     io.Writer
	IsTerminal bool
}

type Factory struct {
	IOStreams *IOStreams
	Keychain  keychain.KeychainAccess
}
