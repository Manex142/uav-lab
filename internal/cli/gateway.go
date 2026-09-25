package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	telemetryv1 "github.com/Manex142/uav-lab/gen/go/telemetry/v1"
	"github.com/Manex142/uav-lab/internal/database"
	"github.com/Manex142/uav-lab/internal/ingest"
	"github.com/Manex142/uav-lab/internal/ledger"
	"github.com/Manex142/uav-lab/internal/server"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"
)

type gatewayOptions struct {
	listenAddr       string
	workersCount     int
	bufferSize       int
	socketBuffer     int
	heartbeatTimeout time.Duration
	checkInterval    time.Duration

	dbHost        string
	dbPort        int
	dbUser        string
	dbPass        string
	dbName        string
	persistEnable bool
	batchSize     int
	flushInterval time.Duration

	webEnable   bool
	httpAddr    string
	wsFrequency int
	homeLat     float64
	homeLon     float64
}

func newGatewayCmd() *cobra.Command {
	opts := &gatewayOptions{}

	cmd := &cobra.Command{
		Use:   "gateway",
		Short: "Start the high-throughput telemetry ingestion gateway",
		Long: `Start the UDP telemetry ingestion gateway with parallel worker pools,
in-memory device ledger, liveness monitoring, batch persistence to TimescaleDB,
and real-time HTTP/WebSocket streaming for web dashboards.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGateway(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.listenAddr, "listen-addr", ":9876", "Dirección y puerto UDP de escucha")
	flags.IntVar(&opts.workersCount, "workers", runtime.NumCPU(), "Número de trabajadores concurrentes (por defecto núcleos CPU)")
	flags.IntVar(&opts.bufferSize, "buffer-size", 10000, "Capacidad de la cola en memoria RAM (número de paquetes)")
	flags.IntVar(&opts.socketBuffer, "socket-buffer", ingest.DefaultSocketBufferSize, "Tamaño del buffer SO_RCVBUF en el kernel (bytes)")
	flags.DurationVar(&opts.heartbeatTimeout, "timeout", 3*time.Second, "Tiempo máximo sin telemetría antes de declarar desconexión")
	flags.DurationVar(&opts.checkInterval, "check-interval", 500*time.Millisecond, "Frecuencia de revisión del monitor de liveness")

	flags.StringVar(&opts.dbHost, "db-host", "127.0.0.1", "Host de TimescaleDB")
	flags.IntVar(&opts.dbPort, "db-port", 5432, "Puerto de TimescaleDB")
	flags.StringVar(&opts.dbUser, "db-user", "uav_admin", "Usuario de TimescaleDB")
	flags.StringVar(&opts.dbPass, "db-pass", "uav_password", "Contraseña de TimescaleDB")
	flags.StringVar(&opts.dbName, "db-name", "uav_telemetry", "Nombre de base de datos TimescaleDB")
	flags.BoolVar(&opts.persistEnable, "persist", true, "Habilitar persistencia batch en TimescaleDB")
	flags.IntVar(&opts.batchSize, "batch-size", 1000, "Tamaño de lote para pgx.CopyFrom")
	flags.DurationVar(&opts.flushInterval, "flush-interval", 100*time.Millisecond, "Intervalo temporal de volcado a base de datos")

	flags.BoolVar(&opts.webEnable, "web", true, "Habilitar servidor HTTP y WebSocket de streaming para el dashboard")
	flags.StringVar(&opts.httpAddr, "http-addr", ":8080", "Dirección y puerto de escucha HTTP/WebSocket")
	flags.IntVar(&opts.wsFrequency, "ws-hz", 10, "Frecuencia de refresco WebSocket para clientes web (Hz)")
	flags.Float64Var(&opts.homeLat, "home-lat", server.DefaultHomeLat, "Latitud de origen para proyección geográfica (WGS84)")
	flags.Float64Var(&opts.homeLon, "home-lon", server.DefaultHomeLon, "Longitud de origen para proyección geográfica (WGS84)")

	return cmd
}

func runGateway(opts *gatewayOptions) error {
	fmt.Println("================================================================")
	fmt.Println(" 🛸 UAV Telemetry Ingestion Gateway (Go High-Throughput Engine)")
	fmt.Println("================================================================")
	fmt.Printf(" Puerto UDP     : %s\n", opts.listenAddr)
	fmt.Printf(" Workers        : %d goroutines en paralelo\n", opts.workersCount)
	fmt.Printf(" Cola RAM       : %d paquetes de capacidad\n", opts.bufferSize)
	fmt.Printf(" Buffer Kernel  : %.1f MB\n", float64(opts.socketBuffer)/(1024*1024))
	fmt.Printf(" Timeout Failsafe: %v (chequeo cada %v)\n", opts.heartbeatTimeout, opts.checkInterval)
	if opts.persistEnable {
		fmt.Printf(" Persistencia BD: ACTIVA en %s:%d/%s (Batch: %d rows / %v)\n",
			opts.dbHost, opts.dbPort, opts.dbName, opts.batchSize, opts.flushInterval)
	} else {
		fmt.Println(" Persistencia BD: DESACTIVADA")
	}
	if opts.webEnable {
		fmt.Printf(" Servidor Web   : ACTIVO en http://localhost%s (WS: /ws/telemetry @ %d Hz)\n", opts.httpAddr, opts.wsFrequency)
	} else {
		fmt.Println(" Servidor Web   : DESACTIVADO")
	}
	fmt.Println(" Presiona [Ctrl+C] para apagar el servidor.")
	fmt.Println("----------------------------------------------------------------")

	var dbPool *pgxpool.Pool
	var batchWriter *database.BatchWriter

	if opts.persistEnable {
		dbCfg := database.Config{
			Host:            opts.dbHost,
			Port:            opts.dbPort,
			User:            opts.dbUser,
			Password:        opts.dbPass,
			Database:        opts.dbName,
			SSLMode:         "disable",
			MaxConns:        20,
			MinConns:        4,
			MaxConnLifetime: 1 * time.Hour,
			MaxConnIdleTime: 30 * time.Minute,
		}

		log.Println("🐘 [Database] Verificando y aplicando migraciones con goose...")
		if err := database.RunMigrations(dbCfg.DSN()); err != nil {
			return fmt.Errorf("error crítico ejecutando migraciones: %w", err)
		}
		log.Println("🐘 [Database] Esquema e hipertablas verificadas.")

		ctxInit, cancelInit := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelInit()

		var err error
		dbPool, err = database.NewPool(ctxInit, dbCfg)
		if err != nil {
			return fmt.Errorf("error crítico conectando con el pool de TimescaleDB: %w", err)
		}
		defer dbPool.Close()

		bwCfg := database.BatchWriterConfig{
			BatchSize:     opts.batchSize,
			FlushInterval: opts.flushInterval,
			ChannelSize:   opts.bufferSize,
			FlushTimeout:  3 * time.Second,
		}
		batchWriter = database.NewBatchWriter(dbPool, bwCfg)
	}

	metrics := &ingest.Metrics{}
	registry := ledger.NewDeviceRegistry()

	var webServer *server.Server
	if opts.webEnable {
		srvCfg := server.ServerConfig{
			ListenAddr: opts.httpAddr,
			HubConfig: server.HubConfig{
				BroadcastHz:   opts.wsFrequency,
				HomeLatitude:  opts.homeLat,
				HomeLongitude: opts.homeLon,
			},
		}
		webServer = server.NewServer(srvCfg, registry, metrics)
		if err := webServer.Start(); err != nil {
			return fmt.Errorf("error crítico arrancando servidor Web: %w", err)
		}
	}

	registry.SetCallbacks(ledger.ConnectionCallbacks{
		OnDiscovered: func(deviceID string, firstSeen time.Time) {
			log.Printf("🟢 [LEDGER] NUEVO DRON DESCUBIERTO: '%s' registrado en el espacio aéreo", deviceID)
			if webServer != nil {
				webServer.Hub().BroadcastEvent(server.LifecycleEventMessage{
					Type:      "LIFECYCLE_EVENT",
					Timestamp: firstSeen.UnixMilli(),
					Event:     "DISCOVERED",
					DeviceID:  deviceID,
				})
			}
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
				deviceID, opts.heartbeatTimeout, lastSeen.Format("15:04:05.000"))
			if webServer != nil {
				webServer.Hub().BroadcastEvent(server.LifecycleEventMessage{
					Type:      "LIFECYCLE_EVENT",
					Timestamp: time.Now().UnixMilli(),
					Event:     "LOST",
					DeviceID:  deviceID,
				})
			}
			if dbPool != nil {
				go func() {
					ctxEvt, cancelEvt := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancelEvt()
					if err := database.RecordDeviceEvent(ctxEvt, dbPool, time.Now().UTC(), deviceID, database.EventLost, map[string]any{
						"last_seen":   lastSeen.UTC().Format(time.RFC3339Nano),
						"timeout_sec": opts.heartbeatTimeout.Seconds(),
					}); err != nil {
						log.Printf("⚠️ [Database] Error registrando evento LOST: %v", err)
					}
				}()
			}
		},
		OnRestored: func(deviceID string, resumedAt time.Time) {
			log.Printf("🔄 [LEDGER] ENLACE RESTAURADO: Dron '%s' vuelve a transmitir telemetría", deviceID)
			if webServer != nil {
				webServer.Hub().BroadcastEvent(server.LifecycleEventMessage{
					Type:      "LIFECYCLE_EVENT",
					Timestamp: resumedAt.UnixMilli(),
					Event:     "RESTORED",
					DeviceID:  deviceID,
				})
			}
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

	livenessMonitor := ledger.NewLivenessMonitor(registry, opts.checkInterval, opts.heartbeatTimeout)
	livenessMonitor.Start()

	var latestDeviceID string
	var latestSeq uint64
	handler := func(record *telemetryv1.TelemetryRecord) {
		latestDeviceID = record.GetDeviceId()
		latestSeq = record.GetSequenceNumber()
		registry.Update(record)
		if batchWriter != nil {
			batchWriter.Enqueue(record)
		}
	}

	listener := ingest.NewListener(opts.listenAddr, opts.socketBuffer, opts.bufferSize, metrics)
	if err := listener.Start(); err != nil {
		return fmt.Errorf("error crítico arrancando Listener UDP: %w", err)
	}

	pool := ingest.NewWorkerPool(opts.workersCount, listener.PacketChan(), metrics, handler)
	pool.Start()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

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

			listener.Stop()
			livenessMonitor.Stop()
			pool.Stop()
			if webServer != nil {
				fmt.Println("   Deteniendo servidor HTTP/WebSocket...")
				ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 2*time.Second)
				_ = webServer.Shutdown(ctxShutdown)
				cancelShutdown()
			}
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
			if finalSnap.PacketsReceived > 0 && totalTime > 0 {
				fmt.Printf("   Tasa media global   : %.1f Hz\n", float64(finalSnap.PacketsReceived)/totalTime)
				fmt.Printf("   Latencia tránsito   : %.3f ms\n", finalSnap.AvgLatencyMs)
			}
			if batchWriter != nil {
				bwStats := batchWriter.Stats()
				fmt.Printf("   Persistidos en BD   : %d registros (%d flushes, %d errores, %d drops)\n",
					bwStats.RecordsPersisted, bwStats.FlushesCount, bwStats.FlushErrors, bwStats.RecordsDropped)
			}
			fmt.Println(" Gateway detenido con éxito.")
			return nil

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

			var wsStatus string
			if webServer != nil {
				wsStatus = fmt.Sprintf(" | WS Clientes: %d", webServer.Hub().ClientCount())
			}

			if snap.PacketsReceived > 0 {
				fmt.Printf("[Gateway] Ingesta: %5.1f Hz | %6.2f KB/s | Latencia: %5.3f ms | Drones ONLINE: %d%s%s | Último: %s (seq=%-6d) | Drops: %d\n",
					currentHz,
					currentKBps,
					snap.AvgLatencyMs,
					activeDrones,
					dbStatus,
					wsStatus,
					latestDeviceID,
					latestSeq,
					snap.PacketsDropped,
				)
			} else {
				fmt.Printf("[Gateway] Esperando datagramas UDP en %s... (Drones ONLINE: %d%s%s)\n", opts.listenAddr, activeDrones, dbStatus, wsStatus)
			}

			prevPackets = snap.PacketsReceived
			prevBytes = snap.BytesReceived
			lastReportTime = now
		}
	}
}
