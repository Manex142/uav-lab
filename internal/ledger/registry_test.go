package ledger_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	telemetryv1 "github.com/Manex142/uav-lab/gen/go/telemetry/v1"
	"github.com/Manex142/uav-lab/internal/ledger"
)

func TestDeviceRegistry_UpdateAndGet(t *testing.T) {
	reg := ledger.NewDeviceRegistry()

	var discoveredCalled uint64
	reg.SetCallbacks(ledger.ConnectionCallbacks{
		OnDiscovered: func(deviceID string, firstSeen time.Time) {
			atomic.AddUint64(&discoveredCalled, 1)
		},
	})

	record := &telemetryv1.TelemetryRecord{
		DeviceId:       "uav-01",
		SequenceNumber: 1,
		TimestampNs:    time.Now().UnixNano(),
		Position:       &telemetryv1.Vector3{X: 10, Y: 20, Z: -15},
		Battery:        &telemetryv1.BatteryState{Percentage: 0.95, VoltageV: 16.4},
		Armed:          true,
	}

	reg.Update(record)

	// Esperar brevemente a la goroutine de callback
	time.Sleep(50 * time.Millisecond)

	state, ok := reg.Get("uav-01")
	if !ok {
		t.Fatalf("se esperaba encontrar uav-01 en el registro")
	}

	if state.DeviceID != "uav-01" {
		t.Errorf("esperado DeviceID uav-01, obtenido %s", state.DeviceID)
	}
	if state.Status != ledger.StatusOnline {
		t.Errorf("esperado Status ONLINE, obtenido %s", state.Status)
	}
	if state.Position.GetX() != 10 {
		t.Errorf("esperado Pos.X 10, obtenido %f", state.Position.GetX())
	}
	if atomic.LoadUint64(&discoveredCalled) != 1 {
		t.Errorf("esperado 1 callback de descubrimiento, obtenidos %d", atomic.LoadUint64(&discoveredCalled))
	}
	if reg.ActiveCount() != 1 {
		t.Errorf("esperado 1 dron activo, obtenidos %d", reg.ActiveCount())
	}
}

func TestDeviceRegistry_LivenessTimeoutAndReconnection(t *testing.T) {
	reg := ledger.NewDeviceRegistry()

	var discoveredCalled uint64
	var lostCalled uint64
	var restoredCalled uint64

	reg.SetCallbacks(ledger.ConnectionCallbacks{
		OnDiscovered: func(deviceID string, firstSeen time.Time) {
			atomic.AddUint64(&discoveredCalled, 1)
		},
		OnLost: func(deviceID string, lastSeen time.Time) {
			atomic.AddUint64(&lostCalled, 1)
		},
		OnRestored: func(deviceID string, resumedAt time.Time) {
			atomic.AddUint64(&restoredCalled, 1)
		},
	})

	// 1. Dron aparece por primera vez: DESCUBIERTO
	reg.Update(&telemetryv1.TelemetryRecord{
		DeviceId:       "uav-heartbeat-test",
		SequenceNumber: 1,
		TimestampNs:    time.Now().UnixNano(),
	})

	time.Sleep(20 * time.Millisecond)
	if atomic.LoadUint64(&discoveredCalled) != 1 {
		t.Errorf("esperado 1 discovered inicial, obtenido %d", atomic.LoadUint64(&discoveredCalled))
	}
	if atomic.LoadUint64(&restoredCalled) != 0 {
		t.Errorf("esperado 0 restored en descubrimiento, obtenido %d", atomic.LoadUint64(&restoredCalled))
	}

	// 2. Comprobar timeout con umbral de 50 ms (esperamos 80 ms para que expire)
	time.Sleep(80 * time.Millisecond)
	timedOut := reg.CheckTimeouts(50 * time.Millisecond)

	if len(timedOut) != 1 || timedOut[0] != "uav-heartbeat-test" {
		t.Fatalf("se esperaba que uav-heartbeat-test tuviera timeout, obtenidos %v", timedOut)
	}

	time.Sleep(20 * time.Millisecond)
	if atomic.LoadUint64(&lostCalled) != 1 {
		t.Errorf("esperado 1 callback lost, obtenido %d", atomic.LoadUint64(&lostCalled))
	}
	if reg.ActiveCount() != 0 {
		t.Errorf("esperados 0 drones activos tras timeout, obtenidos %d", reg.ActiveCount())
	}

	// 3. El dron vuelve a emitir: RESTAURADO
	reg.Update(&telemetryv1.TelemetryRecord{
		DeviceId:       "uav-heartbeat-test",
		SequenceNumber: 2,
		TimestampNs:    time.Now().UnixNano(),
	})

	time.Sleep(20 * time.Millisecond)
	if atomic.LoadUint64(&restoredCalled) != 1 {
		t.Errorf("esperado 1 restored tras reconexión, obtenidos %d", atomic.LoadUint64(&restoredCalled))
	}
	if reg.ActiveCount() != 1 {
		t.Errorf("esperado 1 dron activo tras reconexión, obtenidos %d", reg.ActiveCount())
	}
}

func TestDeviceRegistry_ConcurrentAccess(t *testing.T) {
	reg := ledger.NewDeviceRegistry()
	const numGoroutines = 20
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for g := 0; g < numGoroutines; g++ {
		go func(id int) {
			defer wg.Done()
			devID := "uav-concurrent"

			for i := 0; i < iterations; i++ {
				// Escritura
				reg.Update(&telemetryv1.TelemetryRecord{
					DeviceId:       devID,
					SequenceNumber: uint64(i),
					TimestampNs:    time.Now().UnixNano(),
				})
				// Lectura concurrente
				reg.Get(devID)
				reg.ActiveCount()
			}
		}(g)
	}

	wg.Wait()

	if reg.ActiveCount() != 1 {
		t.Errorf("esperado 1 dron activo, obtenidos %d", reg.ActiveCount())
	}
}
