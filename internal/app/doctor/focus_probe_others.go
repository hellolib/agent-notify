//go:build !windows

package doctor

import (
	"fmt"
	"io"
)

// RunFocusProbe 仅 Windows 支持窗口聚焦探针。
func RunFocusProbe(out io.Writer) error {
	fmt.Fprintln(out, "focus probe 仅在 Windows 上支持")
	return nil
}
