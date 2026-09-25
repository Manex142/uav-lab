package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/Manex142/uav-lab/internal/ingest"
	"github.com/Manex142/uav-lab/internal/ledger"
	"github.com/coder/websocket"
)

// ServerConfig defines network and runtime options for the HTTP/WebSocket gateway.
type ServerConfig struct {
	ListenAddr string
	HubConfig  HubConfig
}

// DefaultServerConfig returns production defaults.
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		ListenAddr: ":8080",
		HubConfig:  DefaultHubConfig(),
	}
}

// Server serves REST APIs and WebSocket real-time telemetry streams.
type Server struct {
	cfg      ServerConfig
	registry *ledger.DeviceRegistry
	metrics  *ingest.Metrics
	hub      *Hub

	httpServer *http.Server
	listener   net.Listener
	startTime  time.Time

	mu sync.Mutex
}

// NewServer initializes the HTTP and WebSocket server.
func NewServer(
	cfg ServerConfig,
	registry *ledger.DeviceRegistry,
	metrics *ingest.Metrics,
) *Server {
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8080"
	}

	hub := NewHub(registry, cfg.HubConfig)

	s := &Server{
		cfg:       cfg,
		registry:  registry,
		metrics:   metrics,
		hub:       hub,
		startTime: time.Now(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws/telemetry", s.handleWebSocket)
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/fleet", s.handleFleet)

	s.httpServer = &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           s.withCORS(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	return s
}

// Hub returns the underlying WebSocket Hub instance.
func (s *Server) Hub() *Hub {
	return s.hub
}

// Start opens the network listener and runs the HTTP server in background.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ln, err := net.Listen("tcp", s.cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("failed to bind HTTP server to %s: %w", s.cfg.ListenAddr, err)
	}
	s.listener = ln

	s.hub.Start()

	go func() {
		if err := s.httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("[Server] HTTP server error: %v", err)
		}
	}()

	return nil
}

// Addr returns the network address the server is listening on.
func (s *Server) Addr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		return s.listener.Addr()
	}
	return nil
}

// Shutdown gracefully terminates HTTP connections and the streaming hub.
func (s *Server) Shutdown(ctx context.Context) error {
	s.hub.Stop()

	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// handleWebSocket handles client connection upgrade and bi-directional lifecycle.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	opts := &websocket.AcceptOptions{
		InsecureSkipVerify: true, // Allow dev frontends from localhost:5173 / localhost:3000
	}

	conn, err := websocket.Accept(w, r, opts)
	if err != nil {
		log.Printf("[Server] WebSocket accept failed: %v", err)
		return
	}

	client := s.hub.RegisterClient(conn)
	defer s.hub.UnregisterClient(client)

	ctx := r.Context()

	// Write pump: consumes messages from client.send channel and writes to socket
	go func() {
		for {
			select {
			case <-ctx.Done():
				return

			case msg, ok := <-client.send:
				if !ok {
					return
				}

				writeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
				err := conn.Write(writeCtx, websocket.MessageText, msg)
				cancel()
				if err != nil {
					return
				}
			}
		}
	}()

	// Read pump: reads incoming client messages (heartbeats, ping/pong, future commands)
	for {
		_, _, err := conn.Read(ctx)
		if err != nil {
			// Disconnect or read error: exit loop to trigger unregister
			break
		}
	}
}

// handleHealth returns real-time gateway ingestion KPIs and server status.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	var snap ingest.Snapshot
	if s.metrics != nil {
		snap = s.metrics.GetSnapshot()
	}

	activeDrones := 0
	if s.registry != nil {
		activeDrones = s.registry.ActiveCount()
	}

	response := map[string]any{
		"status":          "HEALTHY",
		"uptime_sec":      time.Since(s.startTime).Seconds(),
		"active_drones":   activeDrones,
		"ws_clients":      s.hub.ClientCount(),
		"packets_recv":    snap.PacketsReceived,
		"bytes_recv":      snap.BytesReceived,
		"avg_latency_ms":  snap.AvgLatencyMs,
		"dropped_packets": snap.PacketsDropped,
		"decode_errors":   snap.DecodeErrors,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// handleFleet returns the current snapshot of all registered assets via REST.
func (s *Server) handleFleet(w http.ResponseWriter, r *http.Request) {
	devices := s.hub.buildFleetSnapshot(time.Now())

	response := map[string]any{
		"timestamp": time.Now().UnixMilli(),
		"count":     len(devices),
		"devices":   devices,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// withCORS wraps an http.Handler with standard Cross-Origin headers.
func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
