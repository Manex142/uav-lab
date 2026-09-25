package server

import (
	"context"
	"encoding/json"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Manex142/uav-lab/internal/ledger"
	"github.com/coder/websocket"
)

// Default reference coordinates for local flat-earth Cartesian projection (San Sebastián, Spain).
const (
	DefaultHomeLat     = 43.3183
	DefaultHomeLon     = -1.9812
	MetersPerDegreeLat = 111139.0
)

// DeviceTelemetryDTO is the JSON payload representing a single asset's operational state.
type DeviceTelemetryDTO struct {
	ID             string  `json:"id"`
	Status         string  `json:"status"`
	SequenceNumber uint64  `json:"seq"`
	Latitude       float64 `json:"lat"`
	Longitude      float64 `json:"lon"`
	Altitude       float64 `json:"alt"`
	PosX           float64 `json:"pos_x"`
	PosY           float64 `json:"pos_y"`
	PosZ           float64 `json:"pos_z"`
	Yaw            float64 `json:"yaw"`
	Pitch          float64 `json:"pitch"`
	Roll           float64 `json:"roll"`
	Speed          float64 `json:"speed"`
	Armed          bool    `json:"armed"`
	FlightMode     string  `json:"flight_mode"`
	BatteryPct     float32 `json:"battery_pct"`
	BatteryV       float32 `json:"battery_v"`
	BatteryA       float32 `json:"battery_a"`
	BatteryTempC   float32 `json:"battery_temp_c"`
	LastSeenMs     int64   `json:"last_seen_ms"`
}

// FleetSnapshotMessage represents the periodic broadcast containing all active assets.
type FleetSnapshotMessage struct {
	Type      string               `json:"type"` // "FLEET_SNAPSHOT"
	Timestamp int64                `json:"timestamp"`
	Devices   []DeviceTelemetryDTO `json:"devices"`
}

// LifecycleEventMessage represents an immediate asynchronous event (e.g. DISCOVERED, LOST, RESTORED).
type LifecycleEventMessage struct {
	Type      string `json:"type"` // "LIFECYCLE_EVENT"
	Timestamp int64  `json:"timestamp"`
	Event     string `json:"event"`
	DeviceID  string `json:"device_id"`
	Details   string `json:"details,omitempty"`
}

// Client represents an active WebSocket subscriber connection.
type Client struct {
	conn *websocket.Conn
	send chan []byte
}

// HubConfig configures the streaming hub behavior.
type HubConfig struct {
	BroadcastHz    int
	HomeLatitude   float64
	HomeLongitude  float64
	ClientQueueLen int
}

// DefaultHubConfig returns production-ready default parameters.
func DefaultHubConfig() HubConfig {
	return HubConfig{
		BroadcastHz:    10,
		HomeLatitude:   DefaultHomeLat,
		HomeLongitude:  DefaultHomeLon,
		ClientQueueLen: 64,
	}
}

// Hub manages concurrent WebSocket subscribers and broadcasts downsampled telemetry.
type Hub struct {
	cfg      HubConfig
	registry *ledger.DeviceRegistry

	mu      sync.RWMutex
	clients map[*Client]struct{}

	broadcastChan chan []byte
	eventChan     chan []byte
	clientCount   atomic.Int64

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewHub creates a new streaming Hub instance.
func NewHub(registry *ledger.DeviceRegistry, cfg HubConfig) *Hub {
	if cfg.BroadcastHz <= 0 {
		cfg.BroadcastHz = 10
	}
	if cfg.ClientQueueLen <= 0 {
		cfg.ClientQueueLen = 64
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Hub{
		cfg:           cfg,
		registry:      registry,
		clients:       make(map[*Client]struct{}),
		broadcastChan: make(chan []byte, 128),
		eventChan:     make(chan []byte, 64),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start launches the background snapshot ticker and client broadcast dispatcher.
func (h *Hub) Start() {
	h.wg.Add(2)
	go h.runDispatcher()
	go h.runSnapshotTicker()
}

// Stop shuts down the hub and disconnects all clients cleanly.
func (h *Hub) Stop() {
	h.cancel()
	h.wg.Wait()

	h.mu.Lock()
	for client := range h.clients {
		close(client.send)
		_ = client.conn.Close(websocket.StatusGoingAway, "server shutting down")
		delete(h.clients, client)
	}
	h.clientCount.Store(0)
	h.mu.Unlock()
}

// ClientCount returns the number of currently connected WebSocket clients.
func (h *Hub) ClientCount() int64 {
	return h.clientCount.Load()
}

// RegisterClient attaches a newly upgraded WebSocket connection to the broadcast pool.
func (h *Hub) RegisterClient(conn *websocket.Conn) *Client {
	client := &Client{
		conn: conn,
		send: make(chan []byte, h.cfg.ClientQueueLen),
	}

	h.mu.Lock()
	h.clients[client] = struct{}{}
	h.clientCount.Add(1)
	h.mu.Unlock()

	return client
}

// UnregisterClient removes a disconnected client from the hub.
func (h *Hub) UnregisterClient(client *Client) {
	h.mu.Lock()
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		h.clientCount.Add(-1)
		close(client.send)
	}
	h.mu.Unlock()
}

// BroadcastEvent enqueues an asynchronous lifecycle or alarm event for immediate dispatch.
func (h *Hub) BroadcastEvent(event LifecycleEventMessage) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	select {
	case h.eventChan <- data:
	default:
		// Non-blocking drop if internal event buffer is saturated
	}
}

// runDispatcher receives serialized messages and distributes them to connected clients.
func (h *Hub) runDispatcher() {
	defer h.wg.Done()

	for {
		select {
		case <-h.ctx.Done():
			return

		case msg := <-h.eventChan:
			h.fanOut(msg)

		case msg := <-h.broadcastChan:
			h.fanOut(msg)
		}
	}
}

// fanOut distributes a message to all active clients without blocking on slow subscribers.
func (h *Hub) fanOut(msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		select {
		case client.send <- msg:
		default:
			// Client buffer is full (backpressure / slow client): drop message to protect gateway
		}
	}
}

// runSnapshotTicker aggregates active ledger telemetry at the configured downsampled frequency.
func (h *Hub) runSnapshotTicker() {
	defer h.wg.Done()

	interval := time.Second / time.Duration(h.cfg.BroadcastHz)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			return

		case now := <-ticker.C:
			if h.clientCount.Load() == 0 {
				// Avoid unnecessary serialization when no web clients are connected
				continue
			}

			devices := h.buildFleetSnapshot(now)
			msg := FleetSnapshotMessage{
				Type:      "FLEET_SNAPSHOT",
				Timestamp: now.UnixMilli(),
				Devices:   devices,
			}

			data, err := json.Marshal(msg)
			if err != nil {
				continue
			}

			select {
			case h.broadcastChan <- data:
			default:
				// Skip tick if dispatcher is still busy
			}
		}
	}
}

// buildFleetSnapshot converts internal device states into client-ready DTOs with WGS84 coordinates.
func (h *Hub) buildFleetSnapshot(now time.Time) []DeviceTelemetryDTO {
	rawDevices := h.registry.GetAll()
	result := make([]DeviceTelemetryDTO, 0, len(rawDevices))

	metersPerDegreeLon := MetersPerDegreeLat * math.Cos(h.cfg.HomeLatitude*(math.Pi/180.0))

	for _, d := range rawDevices {
		dto := DeviceTelemetryDTO{
			ID:             d.DeviceID,
			Status:         string(d.Status),
			SequenceNumber: d.SequenceNumber,
			Armed:          d.Armed,
			FlightMode:     d.FlightMode.String(),
			LastSeenMs:     now.Sub(d.LastSeen).Milliseconds(),
		}

		if d.Position != nil {
			dto.PosX = d.Position.GetX()
			dto.PosY = d.Position.GetY()
			dto.PosZ = d.Position.GetZ()
			dto.Altitude = math.Abs(d.Position.GetZ())

			// Convert local Cartesian NED meters to WGS84 Geodetic coordinates
			dto.Latitude = h.cfg.HomeLatitude + (dto.PosX / MetersPerDegreeLat)
			dto.Longitude = h.cfg.HomeLongitude + (dto.PosY / metersPerDegreeLon)
		}

		if d.EulerAngles != nil {
			dto.Roll = d.EulerAngles.GetRoll() * (180.0 / math.Pi)
			dto.Pitch = d.EulerAngles.GetPitch() * (180.0 / math.Pi)
			dto.Yaw = d.EulerAngles.GetYaw() * (180.0 / math.Pi)
		}

		if d.LinearVelocity != nil {
			vx := d.LinearVelocity.GetX()
			vy := d.LinearVelocity.GetY()
			vz := d.LinearVelocity.GetZ()
			dto.Speed = math.Sqrt(vx*vx + vy*vy + vz*vz)
		}

		if d.Battery != nil {
			dto.BatteryV = d.Battery.GetVoltageV()
			dto.BatteryA = d.Battery.GetCurrentA()
			dto.BatteryPct = d.Battery.GetPercentage()
			if dto.BatteryPct <= 1.0 {
				dto.BatteryPct *= 100.0
			}
			dto.BatteryTempC = d.Battery.GetTemperatureC()
		}

		result = append(result, dto)
	}

	return result
}
