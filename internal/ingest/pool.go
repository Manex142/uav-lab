package ingest

import (
	"context"
	"log"
	"sync"
	"time"

	telemetryv1 "github.com/Manex142/uav-lab/gen/go/telemetry/v1"
	"google.golang.org/protobuf/proto"
)

// TelemetryHandler es una función callback a la que se le entrega cada paquete
// decodificado con éxito para alimentar el Ledger o la base de datos TimescaleDB.
type TelemetryHandler func(record *telemetryv1.TelemetryRecord)

// WorkerPool gestiona una batería de goroutines en paralelo que consumen
// datagramas crudos del canal, los deserializan con Protobuf y calculan métricas.
type WorkerPool struct {
	workerCount int
	packetChan  <-chan []byte
	metrics     *Metrics
	handler     TelemetryHandler

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Mapa protegido por mutex para detectar pérdidas de paquetes por device_id
	seqMu      sync.Mutex
	lastSeqMap map[string]uint64
	lossCount  uint64
}

// NewWorkerPool inicializa el pool de trabajadores concurrentes.
func NewWorkerPool(
	workerCount int,
	packetChan <-chan []byte,
	metrics *Metrics,
	handler TelemetryHandler,
) *WorkerPool {
	if workerCount <= 0 {
		workerCount = 1
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &WorkerPool{
		workerCount: workerCount,
		packetChan:  packetChan,
		metrics:     metrics,
		handler:     handler,
		ctx:         ctx,
		cancel:      cancel,
		lastSeqMap:  make(map[string]uint64),
	}
}

// Start arranca las W goroutines trabajadoras en segundo plano.
func (p *WorkerPool) Start() {
	p.wg.Add(p.workerCount)
	for i := 0; i < p.workerCount; i++ {
		go p.worker(i)
	}
}

// Stop ordena a los trabajadores detenerse de forma limpia y espera a que terminen.
func (p *WorkerPool) Stop() {
	p.cancel()
	p.wg.Wait()
}

// worker es el bucle individual de cada trabajador en paralelo.
func (p *WorkerPool) worker(id int) {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			// Señal de apagado: salimos
			return

		case payload, ok := <-p.packetChan:
			if !ok {
				// El canal fue cerrado por el Listener: salimos
				return
			}

			p.processPacket(payload)
		}
	}
}

// processPacket deserializa el payload Protobuf y calcula métricas de red.
func (p *WorkerPool) processPacket(payload []byte) {
	p.metrics.IncPacketsReceived()
	p.metrics.AddBytesReceived(len(payload))

	var record telemetryv1.TelemetryRecord
	if err := proto.Unmarshal(payload, &record); err != nil {
		p.metrics.IncDecodeErrors()
		log.Printf("[WorkerPool] Error deserializando Protobuf (%d bytes): %v", len(payload), err)
		return
	}

	// 1. Medir latencia de tránsito en red (Edge -> Gateway)
	if record.GetTimestampNs() > 0 {
		nowNs := time.Now().UnixNano()
		if nowNs >= record.GetTimestampNs() {
			p.metrics.RecordLatency(time.Duration(nowNs - record.GetTimestampNs()))
		}
	}

	// 2. Comprobar secuencia de paquetes para detectar pérdidas en la red
	p.checkSequence(record.GetDeviceId(), record.GetSequenceNumber())

	// 3. Notificar a los suscriptores downstream (Ledger / TimescaleDB)
	if p.handler != nil {
		p.handler(&record)
	}
}

// checkSequence verifica si el número de secuencia recibido es consecutivo.
// Si hay un salto (ej: llega 105 tras 102), significa que 3 paquetes se perdieron en UDP.
func (p *WorkerPool) checkSequence(deviceID string, seq uint64) {
	p.seqMu.Lock()
	defer p.seqMu.Unlock()

	lastSeq, exists := p.lastSeqMap[deviceID]
	if exists {
		if seq > lastSeq+1 {
			missed := seq - (lastSeq + 1)
			p.lossCount += missed
			log.Printf("[Red UDP] ⚠️ Paquetes perdidos en el aire para %s: se esperaban %d..%d (%d perdidos)",
				deviceID, lastSeq+1, seq-1, missed)
		}
	}
	p.lastSeqMap[deviceID] = seq
}

// GetTotalPacketLoss devuelve el número total de paquetes perdidos detectados en red.
func (p *WorkerPool) GetTotalPacketLoss() uint64 {
	p.seqMu.Lock()
	defer p.seqMu.Unlock()
	return p.lossCount
}
