package cli

import (
	"fmt"
	"io"

	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/i18n"
)

const starRepoURL = "https://github.com/hellolib/agent-notify"

// maybeStarPrompt writes a one-time GitHub star invitation to output when the
// user has never been prompted AND stdout is an interactive terminal. It
// returns whether the prompt was shown; persisting that fact is the caller's
// responsibility. It never touches hook-handler code paths.
func maybeStarPrompt(cfg config.Config, output io.Writer, isTTY bool) (shown bool) {
	if cfg.StarPrompted || !isTTY {
		return false
	}
	const rule = "─────────────────────────────────────────────"
	fmt.Fprintf(output, "\n%s\n  ⭐  %s\n  %s\n  %s\n%s\n",
		rule,
		i18n.T("star_prompt.title"),
		i18n.T("star_prompt.body"),
		starRepoURL,
		rule,
	)
	return true
}
