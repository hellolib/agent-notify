package larksuite

import (
	"context"
	"fmt"
	"os"

	"github.com/hellolib/client-tools/app/larksuite/cmdutil"
	"github.com/hellolib/client-tools/app/larksuite/core"
	"github.com/hellolib/client-tools/app/larksuite/keychain"
	"github.com/hellolib/client-tools/utils"
	"golang.org/x/term"
)

// PrepareLarkCLI ensures lark-cli is installed and runs interactive init.
// `lark-cli config init` blocks and prints an ASCII QR code; output is streamed directly.
func PrepareLarkCLI(ctx context.Context) error {
	return prepareLarkCLIWithRunner(ctx, utils.ExecRunner{})
}

func prepareLarkCLIWithRunner(ctx context.Context, runner utils.CommandRunner) error {
	if _, err := runner.LookPath("lark-cli"); err != nil {
		if _, npmErr := runner.LookPath("npm"); npmErr != nil {
			return fmt.Errorf("lark-cli not found and npm not found: install Node.js/npm first")
		}
		if err := runner.Run(ctx, "npm", "install", "-g", "@larksuite/cli"); err != nil {
			return fmt.Errorf("failed to install @larksuite/cli with npm: %w", err)
		}
		if _, err := runner.LookPath("lark-cli"); err != nil {
			return fmt.Errorf("lark-cli still not found after npm install, check PATH")
		}
	}

	if err := runner.Run(ctx, "lark-cli", "config", "init", "--new"); err != nil {
		return fmt.Errorf("lark-cli config init failed: %w", err)
	}
	return nil
}

// ParseCliConfig parses the CLI config.
func ParseCliConfig() (*core.CliConfig, error) {
	f := &cmdutil.Factory{
		Keychain: keychain.Default(),
	}
	f.IOStreams = &cmdutil.IOStreams{
		In:         os.Stdin,
		Out:        os.Stdout,
		ErrOut:     os.Stderr,
		IsTerminal: term.IsTerminal(int(os.Stdin.Fd())),
	}
	return core.RequireConfig(f.Keychain)
}
