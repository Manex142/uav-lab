package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	verbose bool

	rootCmd = &cobra.Command{
		Use:   "uav",
		Short: "UAV Lab Telemetry, Ingestion & Simulation Toolkit",
		Long: `🛸 UAV Lab CLI
A unified high-performance toolkit for UAV swarm telemetry simulation,
UDP ingestion gateway, TimescaleDB persistence, and real-time monitoring.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
)

// Execute runs the root command and exits on error.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Habilitar logging detallado")

	rootCmd.AddCommand(newGatewayCmd())
	rootCmd.AddCommand(newSimCmd())
	rootCmd.AddCommand(newDBCmd())
	rootCmd.AddCommand(newMonitorCmd())
}
