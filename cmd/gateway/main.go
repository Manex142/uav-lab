package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	telemetryv1 "github.com/Manex142/uav-lab/gen/go/telemetry/v1"
	"github.com/Manex142/uav-lab/internal/database"
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

	// Flags de Base de Datos / TimescaleDB
	dbHost := flag.String("db-host", "127.0.0.1", "Host de TimescaleDB")
	dbPort := flag.Int("db-port", 5432, "Puerto de TimescaleDB")
	dbUser := flag.String("db-user", "uav_admin", "Usuario de TimescaleDB")
	dbPass := flag.String("db-pass", "uav_password", "Contraseña de TimescaleDB")
	dbName := flag.String("db-name", "uav_telemetry", "Nombre de base de datos TimescaleDB")
	persistEnable := flag.Bool("persist", true, "Habilitar persistencia batch en TimescaleDB")
	batchSize := flag.Int("batch-size", 1000, "Tamaño de lote para pgx.CopyFrom")
	flushInterval := flag.Duration("flush-interval", 100*time.Millisecond, "Intervalo temporal de volcado a base de datos")
	flag.Parse()

	fmt.Println("================================================================")
	fmt.Println(" 🛸 UAV Telemetry Ingestion Gateway (Go High-Throughput Engine)")
	fmt.Println("================================================================")
	fmt.Printf(" Puerto UDP     : %s\n", *listenAddr)
	fmt.Printf(" Workers        : %d goroutines en paralelo\n", *workersCount)
	fmt.Printf(" Cola RAM       : %d paquetes de capacidad\n", *bufferSize)
	fmt.Printf(" Buffer Kernel  : %.1f MB\n", float64(*socketBuffer)/(1024*1024))
	fmt.Printf(" Timeout Failsafe: %v (chequeo cada %v)\n", *heartbeatTimeout, *checkInterval)
	if *persistEnable {
		fmt.Printf(" Persistencia BD: ACTIVA en %s:%d/%s (Batch: %d rows / %v)\n",
			*dbHost, *dbPort, *dbName, *batchSize, *flushInterval)
	} else {
		fmt.Println(" Persistencia BD: DESACTIVADA")
	}
	fmt.Println(" Presiona [Ctrl+C] para apagar el servidor.")
	fmt.Println("----------------------------------------------------------------")

	// --- Inicialización de Base de Datos y BatchWriter ---
	var dbPool *pgxpool.Pool
	var batchWriter *database.BatchWriter
	if *persistEnable {
		dbCfg := database.Config{
			Host:            *dbHost,
			Port:            *dbPort,
			User:            *dbUser,
			Password:        *dbPass,
			Database:        *dbName,
			SSLMode:         "disable",
			MaxConns:        20,
			MinConns:        4,
			MaxConnLifetime: 1 * time.Hour,
			MaxConnIdleTime: 30 * time.Minute,
		}

		log.Println("🐘 [Database] Verificando y aplicando migraciones con goose...")
		if err := database.RunMigrations(dbCfg.DSN()); err != nil {
			log.Fatalf("Error crítico ejecutando migraciones: %v", err)
		}
		log.Println("🐘 [Database] Esquema e hipertablas verificadas.")

		ctxInit, cancelInit := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelInit()

		var err error
		dbPool, err = database.NewPool(ctxInit, dbCfg)
		if err != nil {
			log.Fatalf("Error crítico conectando con el pool de TimescaleDB: %v", err)
		}
		defer dbPool.Close()

		bwCfg := database.BatchWriterConfig{
			BatchSize:     *batchSize,
			FlushInterval: *flushInterval,
			ChannelSize:   *bufferSize,
			FlushTimeout:  3 * time.Second,
		}
		batchWriter = database.NewBatchWriter(dbPool, bwCfg)
	}

	metrics := &ingest.Metrics{}

	// --- Inicialización del Device Ledger (Registro en Memoria) ---
	registry := ledger.NewDeviceRegistry()
	registry.SetCallbacks(ledger.ConnectionCallbacks{
		OnDiscovered: func(deviceID string, firstSeen time.Time) {
			log.Printf("🟢 [LEDGER] NUEVO DRON DESCUBIERTO: '%s' registrado en el espacio aéreo", deviceID)
			if dbPool != nil {
				go func() {
					ctxEvt, cancelEvt := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancelEvt()
					if err := database.RecordDeviceEvent(ctxEvt, dbPool, firstSeen, deviceID, database.EventDiscovered, map[string]any{
						"source": "udp_listener",
					}); err != nil {
						log.Printf("⚠️ [Database] Error registrando evento DISCOVERED: %v", err)
					}
				}()
			}
		},
		OnLost: func(deviceID string, lastSeen time.Time) {
			log.Printf("🔴 [LEDGER] ⚠️ CONEXIÓN PERDIDA: Dron '%s' sin telemetría (última señal hace >%v a las %s)",
				deviceID, *heartbeatTimeout, lastSeen.Format("15:04:05.000"))
			if dbPool != nil {
				go func() {
					ctxEvt, cancelEvt := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancelEvt()
					if err := database.RecordDeviceEvent(ctxEvt, dbPool, time.Now().UTC(), deviceID, database.EventLost, map[string]any{
						"last_seen":   lastSeen.UTC().Format(time.RFC3339Nano),
						"timeout_sec": heartbeatTimeout.Seconds(),
					}); err != nil {
						log.Printf("⚠️ [Database] Error registrando evento LOST: %v", err)
					}
				}()
			}
		},
		OnRestored: func(deviceID string, resumedAt time.Time) {
			log.Printf("🔄 [LEDGER] ENLACE RESTAURADO: Dron '%s' vuelve a transmitir telemetría", deviceID)
			if dbPool != nil {
				go func() {
					ctxEvt, cancelEvt := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancelEvt()
					if err := database.RecordDeviceEvent(ctxEvt, dbPool, resumedAt.UTC(), deviceID, database.EventRestored, map[string]any{
						"resumed_at": resumedAt.UTC().Format(time.RFC3339Nano),
					}); err != nil {
						log.Printf("⚠️ [Database] Error registrando evento RESTORED: %v", err)
					}
				}()
			}
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
		// Encolar para persistencia masiva en TimescaleDB sin bloquear worker
		if batchWriter != nil {
			batchWriter.Enqueue(record)
		}
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
			// Paso 4: Drenar y vaciar el buffer de persistencia a TimescaleDB
			if batchWriter != nil {
				fmt.Println("   Vaciando buffer de telemetría a TimescaleDB...")
				batchWriter.Close()
			}

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
			if batchWriter != nil {
				bwStats := batchWriter.Stats()
				fmt.Printf("   Persistidos en BD   : %d registros (%d flushes, %d errores, %d drops)\n",
					bwStats.RecordsPersisted, bwStats.FlushesCount, bwStats.FlushErrors, bwStats.RecordsDropped)
			}
			fmt.Println(" Gateway detenido con éxito.")
			return

		case now := <-ticker.C:
			snap := metrics.GetSnapshot()
			deltaSec := now.Sub(lastReportTime).Seconds()

			currentHz := float64(snap.PacketsReceived-prevPackets) / deltaSec
			currentKBps := (float64(snap.BytesReceived-prevBytes) / 1024.0) / deltaSec
			activeDrones := registry.ActiveCount()

			var dbStatus string
			if batchWriter != nil {
				bwStats := batchWriter.Stats()
				dbStatus = fmt.Sprintf(" | BD Persist: %d (%d flushes)", bwStats.RecordsPersisted, bwStats.FlushesCount)
			}

			if snap.PacketsReceived > 0 {
				fmt.Printf("[Gateway] Ingesta: %5.1f Hz | %6.2f KB/s | Latencia: %5.3f ms | Drones ONLINE: %d%s | Último: %s (seq=%-6d) | Drops: %d\n",
					currentHz,
					currentKBps,
					snap.AvgLatencyMs,
					activeDrones,
					dbStatus,
					latestDeviceID,
					latestSeq,
					snap.PacketsDropped,
				)
			} else {
				fmt.Printf("[Gateway] Esperando datagramas UDP en %s... (Drones ONLINE: %d%s)\n", *listenAddr, activeDrones, dbStatus)
			}

			prevPackets = snap.PacketsReceived
			prevBytes = snap.BytesReceived
			lastReportTime = now
		}
	}
}
