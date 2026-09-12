package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Manex142/uav-lab/internal/sim"
)

func main() {
	// --- Configuración por Flags de Terminal ---
	dronesCount := flag.Int("drones", 1, "Número de drones concurrentes a simular")
	targetAddr := flag.String("target-addr", "127.0.0.1:9876", "Dirección UDP destino (host:port)")
	frequencyHz := flag.Int("frequency", 100, "Frecuencia de emisión por dron en Hz")
	enableJitter := flag.Bool("jitter", true, "Desfasar el arranque para evitar micro-bursting")
	radius := flag.Float64("radius", 30.0, "Radio base de la órbita (metros)")
	speed := flag.Float64("speed", 7.0, "Velocidad base de crucero (m/s)")
	altitude := flag.Float64("altitude", 25.0, "Altitud base de vuelo (metros)")
	flag.Parse()

	if *dronesCount <= 0 {
		log.Fatalf("Número de drones inválido: %d (debe ser >= 1)", *dronesCount)
	}
	if *frequencyHz <= 0 {
		log.Fatalf("Frecuencia inválida: %d Hz (debe ser > 0)", *frequencyHz)
	}

	totalExpectedHz := *dronesCount * *frequencyHz

	fmt.Println("================================================================")
	fmt.Println(" 🛸 UAV Telemetry Swarm Simulator (High-Concurrence Engine)")
	fmt.Println("================================================================")
	fmt.Printf(" Flota Drones   : %d UAVs concurrentes (1 goroutine/dron)\n", *dronesCount)
	fmt.Printf(" Destino UDP    : %s\n", *targetAddr)
	fmt.Printf(" Frecuencia UAV : %d Hz por dron (intervalo: %v)\n", *frequencyHz, time.Second/time.Duration(*frequencyHz))
	fmt.Printf(" Rendimiento Obj: ~%d paquetes/segundo (%.1f kHz)\n", totalExpectedHz, float64(totalExpectedHz)/1000.0)
	fmt.Printf(" Phase Jitter   : %v (desfase estocástico en arranque)\n", *enableJitter)
	fmt.Printf(" Trayectoria    : Radio=%.1fm, Vel=%.1fm/s, Altitud=%.1fm\n", *radius, *speed, *altitude)
	fmt.Println(" Presiona [Ctrl+C] para detener la simulación del enjambre.")
	fmt.Println("----------------------------------------------------------------")

	cfg := sim.SwarmConfig{
		DroneCount:   *dronesCount,
		TargetAddr:   *targetAddr,
		FrequencyHz:  *frequencyHz,
		EnableJitter: *enableJitter,
		MinRadius:    *radius * 0.7,
		MaxRadius:    *radius * 1.3,
		MinSpeed:     *speed * 0.8,
		MaxSpeed:     *speed * 1.2,
		BaseAltitude: *altitude,
	}

	swarm := sim.NewSwarm(cfg)
	if err := swarm.Start(); err != nil {
		log.Fatalf("Error crítico iniciando el enjambre: %v", err)
	}

	// --- Captura de Señales del Sistema Operativo para Parada Limpia (Ctrl+C) ---
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// --- Monitorización de Caudal Agregado cada 1 segundo ---
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	startTime := time.Now()
	lastReportTime := time.Now()
	var prevPackets uint64
	var prevBytes uint64

	for {
		select {
		case <-sigChan:
			fmt.Printf("\n\n⏹️  Deteniendo enjambre de %d drones de forma ordenada...\n", *dronesCount)
			swarm.Stop()

			totalTime := time.Since(startTime).Seconds()
			metrics := swarm.Metrics()
			totalPkts := metrics.PacketsSent.Load()
			totalBytes := metrics.BytesSent.Load()
			totalErrors := metrics.NetworkErrors.Load()

			fmt.Printf("   Drones simulados   : %d\n", *dronesCount)
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
			return

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
				*dronesCount,
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
