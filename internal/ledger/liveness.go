package ledger

import (
	"context"
	"sync"
	"time"
)

// DefaultHeartbeatTimeout es el tiempo máximo sin telemetría antes de marcar un dron como caído (3 segundos).
const DefaultHeartbeatTimeout = 3 * time.Second

// DefaultCheckInterval es la frecuencia con la que el vigilante revisa los timeouts (500 ms).
const DefaultCheckInterval = 500 * time.Millisecond

// LivenessMonitor ejecuta un vigilante en segundo plano con time.Ticker
// para auditar la actividad de los drones y declarar pérdidas de conexión.
type LivenessMonitor struct {
	registry      *DeviceRegistry
	checkInterval time.Duration
	timeout       time.Duration

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewLivenessMonitor inicializa el monitor de actividad.
func NewLivenessMonitor(
	registry *DeviceRegistry,
	checkInterval time.Duration,
	timeout time.Duration,
) *LivenessMonitor {
	if checkInterval <= 0 {
		checkInterval = DefaultCheckInterval
	}
	if timeout <= 0 {
		timeout = DefaultHeartbeatTimeout
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &LivenessMonitor{
		registry:      registry,
		checkInterval: checkInterval,
		timeout:       timeout,
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start arranca la goroutine del vigilante en segundo plano.
func (m *LivenessMonitor) Start() {
	m.wg.Add(1)
	go m.watchLoop()
}

// Stop detiene el vigilante de forma limpia.
func (m *LivenessMonitor) Stop() {
	m.cancel()
	m.wg.Wait()
}

// watchLoop es el bucle periódico de vigilancia.
func (m *LivenessMonitor) watchLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.registry.CheckTimeouts(m.timeout)
		}
	}
}
