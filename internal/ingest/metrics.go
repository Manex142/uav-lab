package ingest

import (
	"sync/atomic"
	"time"
)

// Metrics almacena contadores atómicos (lock-free) de alto rendimiento
// para monitorizar el flujo telemétrico sin cuellos de botella entre núcleos CPU.
type Metrics struct {
	packetsReceived uint64
	bytesReceived   uint64
	decodeErrors    uint64
	packetsDropped  uint64

	// Para cálculo de latencia de red (tiempo desde que el dron lo emite hasta que se decodifica)
	totalLatencyNs int64
	latencySamples uint64
}

// IncPacketsReceived incrementa atómicamente el contador de paquetes recibidos.
func (m *Metrics) IncPacketsReceived() {
	atomic.AddUint64(&m.packetsReceived, 1)
}

// AddBytesReceived suma los bytes procesados.
func (m *Metrics) AddBytesReceived(n int) {
	if n > 0 {
		atomic.AddUint64(&m.bytesReceived, uint64(n))
	}
}

// IncDecodeErrors incrementa el contador de errores de deserialización.
func (m *Metrics) IncDecodeErrors() {
	atomic.AddUint64(&m.decodeErrors, 1)
}

// IncPacketsDropped incrementa paquetes descartados si la cola en RAM se llena.
func (m *Metrics) IncPacketsDropped() {
	atomic.AddUint64(&m.packetsDropped, 1)
}

// RecordLatency registra la latencia de tránsito de un paquete recibido.
func (m *Metrics) RecordLatency(transitLatency time.Duration) {
	if transitLatency > 0 {
		atomic.AddInt64(&m.totalLatencyNs, int64(transitLatency))
		atomic.AddUint64(&m.latencySamples, 1)
	}
}

// Snapshot contiene una fotografía inmutable de las métricas en un instante dado.
type Snapshot struct {
	PacketsReceived uint64
	BytesReceived   uint64
	DecodeErrors    uint64
	PacketsDropped  uint64
	AvgLatencyMs    float64
}

// GetSnapshot obtiene los valores acumulados y calcula el promedio de latencia.
func (m *Metrics) GetSnapshot() Snapshot {
	pkts := atomic.LoadUint64(&m.packetsReceived)
	bytes := atomic.LoadUint64(&m.bytesReceived)
	errors := atomic.LoadUint64(&m.decodeErrors)
	dropped := atomic.LoadUint64(&m.packetsDropped)

	var avgLatency float64
	samples := atomic.LoadUint64(&m.latencySamples)
	if samples > 0 {
		totalNs := atomic.LoadInt64(&m.totalLatencyNs)
		avgLatency = (float64(totalNs) / float64(samples)) / 1e6 // convertir a milisegundos
	}

	return Snapshot{
		PacketsReceived: pkts,
		BytesReceived:   bytes,
		DecodeErrors:    errors,
		PacketsDropped:  dropped,
		AvgLatencyMs:    avgLatency,
	}
}
