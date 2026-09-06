package ledger

import (
	"sync"
	"time"

	telemetryv1 "github.com/Manex142/uav-lab/gen/go/telemetry/v1"
)

// ConnectionStatus representa el estado de conectividad en tiempo real del dron.
type ConnectionStatus string

const (
	StatusOnline  ConnectionStatus = "ONLINE"
	StatusOffline ConnectionStatus = "OFFLINE"
)

// DeviceState almacena la fotografía viva del último estado conocido de un dron en RAM.
type DeviceState struct {
	DeviceID       string
	Status         ConnectionStatus
	LastSeen       time.Time
	SequenceNumber uint64
	Position       *telemetryv1.Vector3
	Orientation    *telemetryv1.Quaternion
	EulerAngles    *telemetryv1.EulerAngles
	LinearVelocity *telemetryv1.Vector3
	Battery        *telemetryv1.BatteryState
	Armed          bool
	FlightMode     telemetryv1.FlightMode
}

// ConnectionCallbacks agrupa los callbacks semánticos de ciclo de vida de los drones.
type ConnectionCallbacks struct {
	OnDiscovered func(deviceID string, firstSeen time.Time)
	OnLost       func(deviceID string, lastSeen time.Time)
	OnRestored   func(deviceID string, resumedAt time.Time)
}

// DeviceRegistry es el registro en memoria concurrente y seguro para hilos (thread-safe)
// que almacena el estado de todos los drones de la flota.
type DeviceRegistry struct {
	mu        sync.RWMutex
	devices   map[string]*DeviceState
	callbacks ConnectionCallbacks
}

// NewDeviceRegistry inicializa un registro vacío.
func NewDeviceRegistry() *DeviceRegistry {
	return &DeviceRegistry{
		devices: make(map[string]*DeviceState),
	}
}

// SetCallbacks configura las funciones a ejecutar según los eventos de ciclo de vida.
func (r *DeviceRegistry) SetCallbacks(cb ConnectionCallbacks) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.callbacks = cb
}

// Update actualiza el estado de un dron con cada paquete telemétrico recibido.
func (r *DeviceRegistry) Update(record *telemetryv1.TelemetryRecord) {
	if record == nil || record.GetDeviceId() == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	devID := record.GetDeviceId()

	state, exists := r.devices[devID]
	if !exists {
		// 1. Primer paquete en la historia de este dron: DESCUBRIMIENTO
		state = &DeviceState{
			DeviceID: devID,
			Status:   StatusOnline,
		}
		r.devices[devID] = state
		if r.callbacks.OnDiscovered != nil {
			go r.callbacks.OnDiscovered(devID, now)
		}
	} else if state.Status == StatusOffline {
		// 2. El dron estaba caído (OFFLINE) y ha vuelto a emitir: RECONEXIÓN
		state.Status = StatusOnline
		if r.callbacks.OnRestored != nil {
			go r.callbacks.OnRestored(devID, now)
		}
	}

	// Actualizar campos cinemáticos y de salud en memoria RAM
	state.LastSeen = now
	state.SequenceNumber = record.GetSequenceNumber()
	state.Position = record.GetPosition()
	state.Orientation = record.GetOrientation()
	state.EulerAngles = record.GetEulerAngles()
	state.LinearVelocity = record.GetLinearVelocity()
	state.Battery = record.GetBattery()
	state.Armed = record.GetArmed()
	state.FlightMode = record.GetFlightMode()
}

// Get obtiene una copia del estado actual de un dron concreto.
func (r *DeviceRegistry) Get(deviceID string) (DeviceState, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	dev, exists := r.devices[deviceID]
	if !exists {
		return DeviceState{}, false
	}
	return *dev, true
}

// GetAll devuelve la lista de todos los drones registrados (online y offline).
func (r *DeviceRegistry) GetAll() []DeviceState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]DeviceState, 0, len(r.devices))
	for _, dev := range r.devices {
		result = append(result, *dev)
	}
	return result
}

// ActiveCount devuelve el número de drones actualmente ONLINE.
func (r *DeviceRegistry) ActiveCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, dev := range r.devices {
		if dev.Status == StatusOnline {
			count++
		}
	}
	return count
}

// CheckTimeouts evalúa si algún dron online ha superado el umbral sin emitir telemetría.
// Si expira, cambia su estado a OFFLINE y dispara el evento OnLost.
func (r *DeviceRegistry) CheckTimeouts(timeout time.Duration) []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	var timedOutDevices []string
	now := time.Now()

	for id, dev := range r.devices {
		if dev.Status == StatusOnline && now.Sub(dev.LastSeen) > timeout {
			dev.Status = StatusOffline
			timedOutDevices = append(timedOutDevices, id)
			if r.callbacks.OnLost != nil {
				go r.callbacks.OnLost(id, dev.LastSeen)
			}
		}
	}

	return timedOutDevices
}
