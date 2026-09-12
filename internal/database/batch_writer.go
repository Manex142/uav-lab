package database

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	telemetryv1 "github.com/Manex142/uav-lab/gen/go/telemetry/v1"
)

// telemetryColumns defines the ordered columns matching the telemetry hypertable schema.
var telemetryColumns = []string{
	"time",
	"device_id",
	"sequence_number",
	"pos_x",
	"pos_y",
	"pos_z",
	"rot_w",
	"rot_x",
	"rot_y",
	"rot_z",
	"roll",
	"pitch",
	"yaw",
	"vel_x",
	"vel_y",
	"vel_z",
	"ang_vel_x",
	"ang_vel_y",
	"ang_vel_z",
	"armed",
	"flight_mode",
	"system_status",
	"battery_voltage",
	"battery_current",
	"battery_pct",
	"battery_temp",
}

// BatchWriterConfig defines buffering and flush parameters for TimescaleDB persistence.
type BatchWriterConfig struct {
	BatchSize     int           // Maximum records before triggering a flush (e.g. 1,000)
	FlushInterval time.Duration // Maximum time before flushing pending records (e.g. 100ms)
	ChannelSize   int           // In-memory queue buffer capacity to decouple UDP intake
	FlushTimeout  time.Duration // Timeout for database copy transaction
}

// DefaultBatchWriterConfig provides production-tuned parameters for high-frequency streams.
func DefaultBatchWriterConfig() BatchWriterConfig {
	return BatchWriterConfig{
		BatchSize:     1000,
		FlushInterval: 100 * time.Millisecond,
		ChannelSize:   10000,
		FlushTimeout:  3 * time.Second,
	}
}

// BatchWriterStats exposes operational metrics for observability and benchmarking.
type BatchWriterStats struct {
	RecordsEnqueued  uint64
	RecordsPersisted uint64
	RecordsDropped   uint64
	FlushesCount     uint64
	FlushErrors      uint64
}

// BatchWriter buffers incoming UAV telemetry and writes batches using pgx.CopyFrom.
type BatchWriter struct {
	pool   *pgxpool.Pool
	cfg    BatchWriterConfig
	inChan chan *telemetryv1.TelemetryRecord

	// Metrics
	enqueuedCount  atomic.Uint64
	persistedCount atomic.Uint64
	droppedCount   atomic.Uint64
	flushesCount   atomic.Uint64
	errorsCount    atomic.Uint64

	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
}

// NewBatchWriter creates and starts a background batch persistence worker.
func NewBatchWriter(pool *pgxpool.Pool, cfg BatchWriterConfig) *BatchWriter {
	ctx, cancel := context.WithCancel(context.Background())
	bw := &BatchWriter{
		pool:   pool,
		cfg:    cfg,
		inChan: make(chan *telemetryv1.TelemetryRecord, cfg.ChannelSize),
		ctx:    ctx,
		cancel: cancel,
	}

	bw.wg.Add(1)
	go bw.workerLoop()

	return bw
}

// Enqueue submits a telemetry record to the buffer without blocking the caller.
// Returns false if the buffer is full (packet dropped to preserve UDP ingress throughput).
func (bw *BatchWriter) Enqueue(record *telemetryv1.TelemetryRecord) bool {
	if record == nil {
		return false
	}

	select {
	case bw.inChan <- record:
		bw.enqueuedCount.Add(1)
		return true
	default:
		bw.droppedCount.Add(1)
		return false
	}
}

// Stats returns a snapshot of batch persistence metrics.
func (bw *BatchWriter) Stats() BatchWriterStats {
	return BatchWriterStats{
		RecordsEnqueued:  bw.enqueuedCount.Load(),
		RecordsPersisted: bw.persistedCount.Load(),
		RecordsDropped:   bw.droppedCount.Load(),
		FlushesCount:     bw.flushesCount.Load(),
		FlushErrors:      bw.errorsCount.Load(),
	}
}

// Close gracefully flushes all remaining buffered records and stops the worker.
func (bw *BatchWriter) Close() {
	bw.cancel()
	close(bw.inChan)
	bw.wg.Wait()
}

// workerLoop drains the channel and triggers flushes on size or timer expiration.
func (bw *BatchWriter) workerLoop() {
	defer bw.wg.Done()

	batch := make([]*telemetryv1.TelemetryRecord, 0, bw.cfg.BatchSize)
	ticker := time.NewTicker(bw.cfg.FlushInterval)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := bw.persistBatch(batch); err != nil {
			bw.errorsCount.Add(1)
			log.Printf("⚠️ [BatchWriter] Error flushing %d records to TimescaleDB: %v", len(batch), err)
		} else {
			bw.flushesCount.Add(1)
			bw.persistedCount.Add(uint64(len(batch)))
		}
		// Reset slice length retaining allocated memory capacity
		batch = batch[:0]
	}

	for {
		select {
		case record, ok := <-bw.inChan:
			if !ok {
				// Channel closed: flush remaining records and terminate
				flush()
				return
			}
			batch = append(batch, record)
			if len(batch) >= bw.cfg.BatchSize {
				flush()
			}

		case <-ticker.C:
			flush()
		}
	}
}

// persistBatch executes the high-throughput PostgreSQL Binary Copy protocol.
func (bw *BatchWriter) persistBatch(batch []*telemetryv1.TelemetryRecord) error {
	ctx, cancel := context.WithTimeout(context.Background(), bw.cfg.FlushTimeout)
	defer cancel()

	source := &telemetryCopySource{
		records: batch,
		idx:     -1,
		row:     make([]any, len(telemetryColumns)),
	}

	rowsCopied, err := bw.pool.CopyFrom(
		ctx,
		pgx.Identifier{"telemetry"},
		telemetryColumns,
		source,
	)
	if err != nil {
		return fmt.Errorf("CopyFrom failed: %w", err)
	}

	if int(rowsCopied) != len(batch) {
		return fmt.Errorf("partial copy: copied %d of %d records", rowsCopied, len(batch))
	}

	return nil
}

// telemetryCopySource implements pgx.CopyFromSource with reusable row slice allocations.
type telemetryCopySource struct {
	records []*telemetryv1.TelemetryRecord
	idx     int
	row     []any
}

func (s *telemetryCopySource) Next() bool {
	s.idx++
	return s.idx < len(s.records)
}

func (s *telemetryCopySource) Values() ([]any, error) {
	rec := s.records[s.idx]

	// 1. Time conversion from edge nanoseconds timestamp
	var t time.Time
	if rec.TimestampNs > 0 {
		t = time.Unix(0, rec.TimestampNs).UTC()
	} else {
		t = time.Now().UTC()
	}

	s.row[0] = t
	s.row[1] = rec.GetDeviceId()
	s.row[2] = int64(rec.GetSequenceNumber())

	// Kinematics / Position
	if p := rec.GetPosition(); p != nil {
		s.row[3] = p.GetX()
		s.row[4] = p.GetY()
		s.row[5] = p.GetZ()
	} else {
		s.row[3], s.row[4], s.row[5] = nil, nil, nil
	}

	// Attitude Quaternion
	if q := rec.GetOrientation(); q != nil {
		s.row[6] = q.GetW()
		s.row[7] = q.GetX()
		s.row[8] = q.GetY()
		s.row[9] = q.GetZ()
	} else {
		s.row[6], s.row[7], s.row[8], s.row[9] = nil, nil, nil, nil
	}

	// Euler Angles
	if e := rec.GetEulerAngles(); e != nil {
		s.row[10] = e.GetRoll()
		s.row[11] = e.GetPitch()
		s.row[12] = e.GetYaw()
	} else {
		s.row[10], s.row[11], s.row[12] = nil, nil, nil
	}

	// Velocities
	if v := rec.GetLinearVelocity(); v != nil {
		s.row[13] = v.GetX()
		s.row[14] = v.GetY()
		s.row[15] = v.GetZ()
	} else {
		s.row[13], s.row[14], s.row[15] = nil, nil, nil
	}

	if w := rec.GetAngularVelocity(); w != nil {
		s.row[16] = w.GetX()
		s.row[17] = w.GetY()
		s.row[18] = w.GetZ()
	} else {
		s.row[16], s.row[17], s.row[18] = nil, nil, nil
	}

	// Autopilot State & Power
	s.row[19] = rec.GetArmed()
	s.row[20] = rec.GetFlightMode().String()
	s.row[21] = rec.GetSystemStatus().String()

	if b := rec.GetBattery(); b != nil {
		s.row[22] = b.GetVoltageV()
		s.row[23] = b.GetCurrentA()
		s.row[24] = b.GetPercentage()
		s.row[25] = b.GetTemperatureC()
	} else {
		s.row[22], s.row[23], s.row[24], s.row[25] = nil, nil, nil, nil
	}

	return s.row, nil
}

func (s *telemetryCopySource) Err() error {
	return nil
}
