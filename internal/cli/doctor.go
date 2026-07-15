package cli

import (
	"fmt"

	"github.com/hellolib/agent-notify/internal/agentintegrations"
	"github.com/hellolib/agent-notify/internal/app/doctor"
	"github.com/spf13/cobra"
)

func newDoctorCmd(streams Streams) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check current notification setup",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := doctor.NewService(
				doctor.WithClaudeIntegration(agentintegrations.NewClaudeIntegration()),
				doctor.WithCodexIntegration(agentintegrations.NewCodexIntegration()),
				doctor.WithZcodeIntegration(agentintegrations.NewZcodeIntegration()),
				doctor.WithGrokIntegration(agentintegrations.NewGrokIntegration()),
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
}

// streamOutputWriter implements doctor.OutputWriter
type streamOutputWriter struct {
	streams Streams
}

func (w *streamOutputWriter) Writef(format string, args ...any) {
	fmt.Fprintf(w.streams.Stdout, format, args...)
}
