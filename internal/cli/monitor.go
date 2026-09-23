package cli

import (
	"fmt"
	"io"
	"log"
	"runtime"
	"time"

	tea "charm.land/bubbletea/v2"
	telemetryv1 "github.com/Manex142/uav-lab/gen/go/telemetry/v1"
	"github.com/Manex142/uav-lab/internal/ingest"
	"github.com/Manex142/uav-lab/internal/ledger"
	"github.com/Manex142/uav-lab/internal/tui"
	"github.com/spf13/cobra"
)

type monitorOptions struct {
	listenAddr       string
	workersCount     int
	bufferSize       int
	heartbeatTimeout time.Duration
	checkInterval    time.Duration
}

func newMonitorCmd() *cobra.Command {
	opts := &monitorOptions{}

	cmd := &cobra.Command{
		Use:   "monitor",
		Short: "Launch interactive terminal dashboard for telemetry & swarm monitoring",
		Long: `Start an interactive Terminal User Interface (TUI) powered by Bubble Tea
to monitor high-frequency telemetry ingress, fleet status, and network latencies in real time.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMonitor(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.listenAddr, "listen-addr", ":9876", "Dirección y puerto UDP de escucha")
	flags.IntVar(&opts.workersCount, "workers", runtime.NumCPU(), "Número de trabajadores concurrentes")
	flags.IntVar(&opts.bufferSize, "buffer-size", 10000, "Capacidad de la cola en memoria RAM")
	flags.DurationVar(&opts.heartbeatTimeout, "timeout", 3*time.Second, "Tiempo máximo sin telemetría antes de declarar desconexión")
	flags.DurationVar(&opts.checkInterval, "check-interval", 500*time.Millisecond, "Frecuencia de chequeo del liveness monitor")

	return cmd
}

func runMonitor(opts *monitorOptions) error {
	// Silenciar el logger estándar para evitar que llamadas a log.Printf en goroutines
	// de fondo colisionen con la terminal en modo altscreen de la TUI.
	origLogWriter := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(origLogWriter)

	metrics := &ingest.Metrics{}
	registry := ledger.NewDeviceRegistry()

	eventChan := make(chan tui.EventItem, 100)
	registry.SetCallbacks(ledger.ConnectionCallbacks{
		OnDiscovered: func(deviceID string, firstSeen time.Time) {
			select {
			case eventChan <- tui.EventItem{
				Time:     firstSeen,
				Type:     "ONLINE",
				DeviceID: deviceID,
				Text:     "contacto establecido",
			}:
			default:
			}
		},
		OnLost: func(deviceID string, lastSeen time.Time) {
			select {
			case eventChan <- tui.EventItem{
				Time:     time.Now(),
				Type:     "LOST",
				DeviceID: deviceID,
				Text:     fmt.Sprintf("sin señal (>%.1fs)", opts.heartbeatTimeout.Seconds()),
			}:
			default:
			}
		},
		OnRestored: func(deviceID string, resumedAt time.Time) {
			select {
			case eventChan <- tui.EventItem{
				Time:     resumedAt,
				Type:     "RESTORED",
				DeviceID: deviceID,
				Text:     "enlace recuperado",
			}:
			default:
			}
		},
	})

	livenessMonitor := ledger.NewLivenessMonitor(registry, opts.checkInterval, opts.heartbeatTimeout)
	livenessMonitor.Start()
	defer livenessMonitor.Stop()

	handler := func(record *telemetryv1.TelemetryRecord) {
		registry.Update(record)
	}

	listener := ingest.NewListener(opts.listenAddr, ingest.DefaultSocketBufferSize, opts.bufferSize, metrics)
	if err := listener.Start(); err != nil {
		return fmt.Errorf("error arrancando listener UDP en %s: %w", opts.listenAddr, err)
	}
	defer listener.Stop()

	pool := ingest.NewWorkerPool(opts.workersCount, listener.PacketChan(), metrics, handler)
	pool.SetOnPacketLoss(func(deviceID string, expectedStart, expectedEnd, missed uint64) {
		select {
		case eventChan <- tui.EventItem{
			Time:     time.Now(),
			Type:     "LOSS",
			DeviceID: deviceID,
			Text:     fmt.Sprintf("%d pkt perdido(s) en red", missed),
		}:
		default:
		}
	})
	pool.Start()
	defer pool.Stop()

	model := tui.NewModel(registry, metrics, eventChan)
	p := tea.NewProgram(model)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error ejecutando TUI: %w", err)
	}

	fmt.Println("Monitor TUI finalizado limpiamente.")
	return nil
}
