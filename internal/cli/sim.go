package cli

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Manex142/uav-lab/internal/sim"
	"github.com/spf13/cobra"
)

type simOptions struct {
	dronesCount  int
	targetAddr   string
	frequencyHz  int
	enableJitter bool
	radius       float64
	speed        float64
	altitude     float64
}

func newSimCmd() *cobra.Command {
	opts := &simOptions{}

	cmd := &cobra.Command{
		Use:   "sim",
		Short: "Run synthetic UAV telemetry simulation",
		Long: `Simulate high-frequency synthetic telemetry from one or more concurrent UAVs
emitting Protocol Buffers over UDP with orbital motion kinematics and jitter.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSim(opts)
		},
	}

	flags := cmd.Flags()
	flags.IntVarP(&opts.dronesCount, "drones", "d", 1, "Número de drones concurrentes a simular")
	flags.StringVarP(&opts.targetAddr, "target-addr", "t", "127.0.0.1:9876", "Dirección UDP destino (host:port)")
	flags.IntVarP(&opts.frequencyHz, "frequency", "f", 100, "Frecuencia de emisión por dron en Hz")
	flags.BoolVar(&opts.enableJitter, "jitter", true, "Desfasar el arranque para evitar micro-bursting")
	flags.Float64Var(&opts.radius, "radius", 30.0, "Radio base de la órbita (metros)")
	flags.Float64Var(&opts.speed, "speed", 7.0, "Velocidad base de crucero (m/s)")
	flags.Float64Var(&opts.altitude, "altitude", 25.0, "Altitud base de vuelo (metros)")

	return cmd
}

func runSim(opts *simOptions) error {
	if opts.dronesCount <= 0 {
		return fmt.Errorf("número de drones inválido: %d (debe ser >= 1)", opts.dronesCount)
	}
	if opts.frequencyHz <= 0 {
		return fmt.Errorf("frecuencia inválida: %d Hz (debe ser > 0)", opts.frequencyHz)
	}

	totalExpectedHz := opts.dronesCount * opts.frequencyHz

	fmt.Println("================================================================")
	fmt.Println(" 🛸 UAV Telemetry Swarm Simulator (High-Concurrence Engine)")
	fmt.Println("================================================================")
	fmt.Printf(" Flota Drones   : %d UAVs concurrentes (1 goroutine/dron)\n", opts.dronesCount)
	fmt.Printf(" Destino UDP    : %s\n", opts.targetAddr)
	fmt.Printf(" Frecuencia UAV : %d Hz por dron (intervalo: %v)\n", opts.frequencyHz, time.Second/time.Duration(opts.frequencyHz))
	fmt.Printf(" Rendimiento Obj: ~%d paquetes/segundo (%.1f kHz)\n", totalExpectedHz, float64(totalExpectedHz)/1000.0)
	fmt.Printf(" Phase Jitter   : %v (desfase estocástico en arranque)\n", opts.enableJitter)
	fmt.Printf(" Trayectoria    : Radio=%.1fm, Vel=%.1fm/s, Altitud=%.1fm\n", opts.radius, opts.speed, opts.altitude)
	fmt.Println(" Presiona [Ctrl+C] para detener la simulación del enjambre.")
	fmt.Println("----------------------------------------------------------------")

	cfg := sim.SwarmConfig{
		DroneCount:   opts.dronesCount,
		TargetAddr:   opts.targetAddr,
		FrequencyHz:  opts.frequencyHz,
		EnableJitter: opts.enableJitter,
		MinRadius:    opts.radius * 0.7,
		MaxRadius:    opts.radius * 1.3,
		MinSpeed:     opts.speed * 0.8,
		MaxSpeed:     opts.speed * 1.2,
		BaseAltitude: opts.altitude,
	}

	swarm := sim.NewSwarm(cfg)
	if err := swarm.Start(); err != nil {
		return fmt.Errorf("error crítico iniciando el enjambre: %w", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	startTime := time.Now()
	lastReportTime := time.Now()
	var prevPackets uint64
	var prevBytes uint64

	for {
		select {
		case <-sigChan:
			fmt.Printf("\n\n⏹️  Deteniendo enjambre de %d drones de forma ordenada...\n", opts.dronesCount)
			swarm.Stop()

			totalTime := time.Since(startTime).Seconds()
			metrics := swarm.Metrics()
			totalPkts := metrics.PacketsSent.Load()
			totalBytes := metrics.BytesSent.Load()
			totalErrors := metrics.NetworkErrors.Load()

			fmt.Printf("   Drones simulados   : %d\n", opts.dronesCount)
			fmt.Printf("   Tiempo de vuelo    : %.2f segundos\n", totalTime)
			fmt.Printf("   Paquetes emitidos  : %d\n", totalPkts)
			fmt.Printf("   Bytes transmitidos : %d (%.2f MB)\n", totalBytes, float64(totalBytes)/(1024.0*1024.0))
			fmt.Printf("   Errores de red UDP : %d\n", totalErrors)
			if totalTime > 0 {
				fmt.Printf("   Tasa media global  : %.2f Hz\n", float64(totalPkts)/totalTime)
				fmt.Printf("   Ancho de banda     : %.2f KB/s (%.2f Mbit/s)\n",
					(float64(totalBytes)/1024.0)/totalTime,
					(float64(totalBytes)*8.0)/(1024.0*1024.0*totalTime))
			}
			fmt.Println(" Enjambre detenido con éxito.")
			return nil

		case now := <-ticker.C:
			deltaSec := now.Sub(lastReportTime).Seconds()
			metrics := swarm.Metrics()
			totalPkts := metrics.PacketsSent.Load()
			totalBytes := metrics.BytesSent.Load()
			netErrors := metrics.NetworkErrors.Load()

			currentHz := float64(totalPkts-prevPackets) / deltaSec
			currentKBps := (float64(totalBytes-prevBytes) / 1024.0) / deltaSec

			fmt.Printf("[Swarm] Emisión: %7.1f Hz | %7.2f KB/s | Drones: %3d | Total: %-7d pkts (%.2f MB) | Errs: %d\n",
				currentHz,
				currentKBps,
				opts.dronesCount,
				totalPkts,
				float64(totalBytes)/(1024.0*1024.0),
				netErrors,
			)

			prevPackets = totalPkts
			prevBytes = totalBytes
			lastReportTime = now
		}
	}
}
