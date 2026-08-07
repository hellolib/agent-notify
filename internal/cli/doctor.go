package cli

import (
	"fmt"

	"github.com/hellolib/agent-notify/internal/agentintegrations"
	"github.com/hellolib/agent-notify/internal/app/doctor"
	"github.com/spf13/cobra"
)

func newDoctorCmd(streams Streams) *cobra.Command {
	var focusProbe bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check current notification setup",
		RunE: func(cmd *cobra.Command, args []string) error {
			if focusProbe {
				return doctor.RunFocusProbe(cmd.OutOrStdout())
			}
			svc := doctor.NewService(
				doctor.WithClaudeIntegration(agentintegrations.NewClaudeIntegration()),
				doctor.WithCodexIntegration(agentintegrations.NewCodexIntegration()),
				doctor.WithZcodeIntegration(agentintegrations.NewZcodeIntegration()),
				doctor.WithGrokIntegration(agentintegrations.NewGrokIntegration()),
				doctor.WithDroidIntegration(agentintegrations.NewDroidIntegration()),
				doctor.WithOpenCodeIntegration(agentintegrations.NewOpenCodeIntegration()),
			)
			result, err := svc.Run()
			if err != nil {
				return err
			}

			// Create output writer
			output := &streamOutputWriter{streams: streams}
			svc.Print(output, result)
			return nil
		},
	}
	cmd.Flags().BoolVar(&focusProbe, "focus-probe", false, "打印 Windows 点击聚焦解析诊断")
	return cmd
}

// streamOutputWriter implements doctor.OutputWriter
type streamOutputWriter struct {
	streams Streams
}

func (w *streamOutputWriter) Writef(format string, args ...any) {
	fmt.Fprintf(w.streams.Stdout, format, args...)
}
