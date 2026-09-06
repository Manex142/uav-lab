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
	"github.com/Manex142/uav-lab/internal/ledger"
)

func main() {
	// --- Configuración por Flags ---
	listenAddr := flag.String("listen-addr", ":9876", "Dirección y puerto UDP de escucha")
	workersCount := flag.Int("workers", runtime.NumCPU(), "Número de trabajadores concurrentes (por defecto núcleos CPU)")
	bufferSize := flag.Int("buffer-size", 10000, "Capacidad de la cola en memoria RAM (número de paquetes)")
	socketBuffer := flag.Int("socket-buffer", ingest.DefaultSocketBufferSize, "Tamaño del buffer SO_RCVBUF en el kernel (bytes)")
	heartbeatTimeout := flag.Duration("timeout", 3*time.Second, "Tiempo máximo sin telemetría antes de declarar desconexión")
	checkInterval := flag.Duration("check-interval", 500*time.Millisecond, "Frecuencia de revisión del monitor de liveness")
	flag.Parse()

	fmt.Println("================================================================")
	fmt.Println(" 🛸 UAV Telemetry Ingestion Gateway (Go High-Throughput Engine)")
	fmt.Println("================================================================")
	fmt.Printf(" Puerto UDP     : %s\n", *listenAddr)
	fmt.Printf(" Workers        : %d goroutines en paralelo\n", *workersCount)
	fmt.Printf(" Cola RAM       : %d paquetes de capacidad\n", *bufferSize)
	fmt.Printf(" Buffer Kernel  : %.1f MB\n", float64(*socketBuffer)/(1024*1024))
	fmt.Printf(" Timeout Failsafe: %v (chequeo cada %v)\n", *heartbeatTimeout, *checkInterval)
	fmt.Println(" Presiona [Ctrl+C] para apagar el servidor.")
	fmt.Println("----------------------------------------------------------------")

	metrics := &ingest.Metrics{}

	// --- Inicialización del Device Ledger (Registro en Memoria) ---
	registry := ledger.NewDeviceRegistry()
	registry.SetCallbacks(ledger.ConnectionCallbacks{
		OnDiscovered: func(deviceID string, firstSeen time.Time) {
			log.Printf("🟢 [LEDGER] NUEVO DRON DESCUBIERTO: '%s' registrado en el espacio aéreo", deviceID)
		},
		OnLost: func(deviceID string, lastSeen time.Time) {
			log.Printf("🔴 [LEDGER] ⚠️ CONEXIÓN PERDIDA: Dron '%s' sin telemetría (última señal hace >%v a las %s)",
				deviceID, *heartbeatTimeout, lastSeen.Format("15:04:05.000"))
		},
		OnRestored: func(deviceID string, resumedAt time.Time) {
			log.Printf("🔄 [LEDGER] ENLACE RESTAURADO: Dron '%s' vuelve a transmitir telemetría", deviceID)
		},
	})

	// --- Inicialización del Vigilante en Segundo Plano (Liveness Monitor) ---
	livenessMonitor := ledger.NewLivenessMonitor(registry, *checkInterval, *heartbeatTimeout)
	livenessMonitor.Start()

	// Handler al que los workers entregan cada paquete decodificado
	var latestDeviceID string
	var latestSeq uint64
	handler := func(record *telemetryv1.TelemetryRecord) {
		latestDeviceID = record.GetDeviceId()
		latestSeq = record.GetSequenceNumber()
		// Actualizar la foto fija del dron en RAM
		registry.Update(record)
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
			// Paso 2: Detener vigilante de liveness
			livenessMonitor.Stop()
			// Paso 3: Esperar a que los workers vacíen los paquetes pendientes en el canal
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
			activeDrones := registry.ActiveCount()

			if snap.PacketsReceived > 0 {
				fmt.Printf("[Gateway] Ingesta: %5.1f Hz | %6.2f KB/s | Latencia: %5.3f ms | Drones ONLINE: %d | Último: %s (seq=%-6d) | Drops: %d\n",
					currentHz,
					currentKBps,
					snap.AvgLatencyMs,
					activeDrones,
					latestDeviceID,
					latestSeq,
					snap.PacketsDropped,
				)
			} else {
				fmt.Printf("[Gateway] Esperando datagramas UDP en %s... (Drones ONLINE: %d)\n", *listenAddr, activeDrones)
			}

			prevPackets = snap.PacketsReceived
			prevBytes = snap.BytesReceived
			lastReportTime = now
		}
	}
}
