package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	telemetryv1 "github.com/Manex142/uav-lab/gen/go/telemetry/v1"
	"github.com/Manex142/uav-lab/internal/ingest"
)

func main() {
	// --- Configuración por Flags ---
	listenAddr := flag.String("listen-addr", ":9876", "Dirección y puerto UDP de escucha")
	workersCount := flag.Int("workers", runtime.NumCPU(), "Número de trabajadores concurrentes (por defecto núcleos CPU)")
	bufferSize := flag.Int("buffer-size", 10000, "Capacidad de la cola en memoria RAM (número de paquetes)")
	socketBuffer := flag.Int("socket-buffer", ingest.DefaultSocketBufferSize, "Tamaño del buffer SO_RCVBUF en el kernel (bytes)")
	flag.Parse()

	fmt.Println("================================================================")
	fmt.Println(" 🛸 UAV Telemetry Ingestion Gateway (Go High-Throughput Engine)")
	fmt.Println("================================================================")
	fmt.Printf(" Puerto UDP     : %s\n", *listenAddr)
	fmt.Printf(" Workers        : %d goroutines en paralelo\n", *workersCount)
	fmt.Printf(" Cola RAM       : %d paquetes de capacidad\n", *bufferSize)
	fmt.Printf(" Buffer Kernel  : %.1f MB\n", float64(*socketBuffer)/(1024*1024))
	fmt.Println(" Presiona [Ctrl+C] para apagar el servidor.")
	fmt.Println("----------------------------------------------------------------")

	metrics := &ingest.Metrics{}

	// Handler que recibe cada paquete decodificado
	// (En siguientes tareas conectará con el Device Ledger y TimescaleDB)
	var latestDeviceID string
	var latestSeq uint64
	handler := func(record *telemetryv1.TelemetryRecord) {
		latestDeviceID = record.GetDeviceId()
		latestSeq = record.GetSequenceNumber()
	}

	// 1. Arrancar el Listener UDP
	listener := ingest.NewListener(*listenAddr, *socketBuffer, *bufferSize, metrics)
	if err := listener.Start(); err != nil {
		log.Fatalf("Error crítico arrancando Listener UDP: %v", err)
	}

	// 2. Arrancar el Pool de Workers
	pool := ingest.NewWorkerPool(*workersCount, listener.PacketChan(), metrics, handler)
	pool.Start()

	// 3. Captura de señales para apagado limpio (Graceful Shutdown)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// 4. Bucle de monitorización: imprime estadísticas cada 1 segundo
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	startTime := time.Now()
	var prevPackets uint64
	var prevBytes uint64
	lastReportTime := time.Now()

	for {
		select {
		case <-sigChan:
			fmt.Println("\n\n⏹️  Apagando Gateway de forma ordenada...")

			// Paso 1: Dejar de aceptar paquetes de red
			listener.Stop()
			// Paso 2: Esperar a que los workers procesen los paquetes pendientes en el canal
			pool.Stop()

			totalTime := time.Since(startTime).Seconds()
			finalSnap := metrics.GetSnapshot()
			fmt.Printf("   Tiempo activo       : %.2f segundos\n", totalTime)
			fmt.Printf("   Paquetes procesados : %d\n", finalSnap.PacketsReceived)
			fmt.Printf("   Bytes ingeridos     : %d (%.2f KB)\n", finalSnap.BytesReceived, float64(finalSnap.BytesReceived)/1024.0)
			fmt.Printf("   Errores de decode   : %d\n", finalSnap.DecodeErrors)
			fmt.Printf("   Paquetes descartados: %d\n", finalSnap.PacketsDropped)
			fmt.Printf("   Pérdidas en red UDP : %d\n", pool.GetTotalPacketLoss())
			if finalSnap.PacketsReceived > 0 {
				fmt.Printf("   Tasa media global   : %.1f Hz\n", float64(finalSnap.PacketsReceived)/totalTime)
				fmt.Printf("   Latencia tránsito   : %.3f ms\n", finalSnap.AvgLatencyMs)
			}
			fmt.Println(" Gateway detenido con éxito.")
			return

		case now := <-ticker.C:
			snap := metrics.GetSnapshot()
			deltaSec := now.Sub(lastReportTime).Seconds()

			currentHz := float64(snap.PacketsReceived-prevPackets) / deltaSec
			currentKBps := (float64(snap.BytesReceived-prevBytes) / 1024.0) / deltaSec

			if snap.PacketsReceived > 0 {
				fmt.Printf("[Gateway] Ingesta: %5.1f Hz | %6.2f KB/s | Latencia: %5.3f ms | Último: %s (seq=%-6d) | Drops: %d\n",
					currentHz,
					currentKBps,
					snap.AvgLatencyMs,
					latestDeviceID,
					latestSeq,
					snap.PacketsDropped,
				)
			} else {
				fmt.Printf("[Gateway] Esperando datagramas UDP en %s... (0 paquetes)\n", *listenAddr)
			}

			prevPackets = snap.PacketsReceived
			prevBytes = snap.BytesReceived
			lastReportTime = now
		}
	}
}
