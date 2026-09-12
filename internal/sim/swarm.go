package sim

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net"
	"sync"
	"sync/atomic"
	"time"

	telemetryv1 "github.com/Manex142/uav-lab/gen/go/telemetry/v1"
	"google.golang.org/protobuf/proto"
)

// SwarmConfig defines parameters for multi-UAV synthetic workload generation.
type SwarmConfig struct {
	DroneCount    int           // Total number of concurrent UAV simulators
	TargetAddr    string        // Destination UDP address (e.g. 127.0.0.1:9876)
	FrequencyHz   int           // Telemetry frequency per drone in Hz (e.g. 100)
	EnableJitter  bool          // Stagger initial emissions to avoid micro-bursting
	MinRadius     float64       // Minimum orbit radius (meters)
	MaxRadius     float64       // Maximum orbit radius (meters)
	MinSpeed      float64       // Minimum horizontal cruise speed (m/s)
	MaxSpeed      float64       // Maximum horizontal cruise speed (m/s)
	BaseAltitude  float64       // Base flight altitude (meters)
}

// DefaultSwarmConfig returns production-tested defaults for load testing.
func DefaultSwarmConfig() SwarmConfig {
	return SwarmConfig{
		DroneCount:   10,
		TargetAddr:   "127.0.0.1:9876",
		FrequencyHz:  100,
		EnableJitter: true,
		MinRadius:    20.0,
		MaxRadius:    50.0,
		MinSpeed:     5.0,
		MaxSpeed:     12.0,
		BaseAltitude: 25.0,
	}
}

// SwarmMetrics tracks aggregated real-time transmission statistics.
type SwarmMetrics struct {
	PacketsSent    atomic.Uint64
	BytesSent      atomic.Uint64
	NetworkErrors  atomic.Uint64
}

// Swarm manages a fleet of concurrent simulated drones.
type Swarm struct {
	cfg     SwarmConfig
	metrics *SwarmMetrics
	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewSwarm initializes the swarm orchestrator.
func NewSwarm(cfg SwarmConfig) *Swarm {
	ctx, cancel := context.WithCancel(context.Background())
	return &Swarm{
		cfg:     cfg,
		metrics: &SwarmMetrics{},
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Metrics returns the active telemetry transmission metrics.
func (s *Swarm) Metrics() *SwarmMetrics {
	return s.metrics
}

// Start spawns a dedicated goroutine for each drone in the swarm.
func (s *Swarm) Start() error {
	raddr, err := net.ResolveUDPAddr("udp", s.cfg.TargetAddr)
	if err != nil {
		return fmt.Errorf("failed to resolve UDP target address %s: %w", s.cfg.TargetAddr, err)
	}

	interval := time.Second / time.Duration(s.cfg.FrequencyHz)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := 1; i <= s.cfg.DroneCount; i++ {
		deviceID := fmt.Sprintf("uav-%03d", i)

		// Randomize drone flight dynamics
		radius := s.cfg.MinRadius + rng.Float64()*(s.cfg.MaxRadius-s.cfg.MinRadius)
		speed := s.cfg.MinSpeed + rng.Float64()*(s.cfg.MaxSpeed-s.cfg.MinSpeed)
		alt := s.cfg.BaseAltitude + (float64(i%5) * 5.0) // Varied altitudes across tiers

		// Compute initial phase jitter (spread evenly across 1 interval period)
		var jitter time.Duration
		if s.cfg.EnableJitter && s.cfg.DroneCount > 1 {
			jitter = time.Duration(float64(interval) * (float64(i) / float64(s.cfg.DroneCount)))
		}

		s.wg.Add(1)
		go s.runDrone(deviceID, raddr, interval, jitter, radius, speed, alt)
	}

	return nil
}

// Stop signals all drones to stop transmission and waits for goroutines to exit.
func (s *Swarm) Stop() {
	s.cancel()
	s.wg.Wait()
}

// runDrone simulates a single autonomous UAV executing a 3D orbit and broadcasting UDP datagrams.
func (s *Swarm) runDrone(
	deviceID string,
	raddr *net.UDPAddr,
	interval time.Duration,
	jitter time.Duration,
	radius, speed, altitude float64,
) {
	defer s.wg.Done()

	// 1. Open dedicated UDP socket for this drone
	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		s.metrics.NetworkErrors.Add(1)
		return
	}
	defer conn.Close()

	// 2. Apply initial phase jitter before starting ticker
	if jitter > 0 {
		select {
		case <-time.After(jitter):
		case <-s.ctx.Done():
			return
		}
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	startTime := time.Now()
	var seq uint64

	for {
		select {
		case <-s.ctx.Done():
			return

		case now := <-ticker.C:
			seq++
			elapsedSec := now.Sub(startTime).Seconds()

			record := computeSyntheticRecord(deviceID, seq, now.UnixNano(), elapsedSec, radius, speed, altitude)

			payload, err := proto.Marshal(record)
			if err != nil {
				continue
			}

			n, err := conn.Write(payload)
			if err != nil {
				s.metrics.NetworkErrors.Add(1)
				continue
			}

			s.metrics.PacketsSent.Add(1)
			s.metrics.BytesSent.Add(uint64(n))
		}
	}
}

// computeSyntheticRecord generates physical kinematics for a 3D circular trajectory.
func computeSyntheticRecord(
	deviceID string,
	seq uint64,
	timestampNs int64,
	elapsedSec float64,
	radius, speed, altitude float64,
) *telemetryv1.TelemetryRecord {
	omega := speed / radius
	theta := omega * elapsedSec

	// Posición NED (Z negativo = altitud sobre el terreno)
	x := radius * math.Cos(theta)
	y := radius * math.Sin(theta)
	z := -(altitude + 2.0*math.Sin(0.2*theta))

	// Velocidades lineales
	vx := -radius * omega * math.Sin(theta)
	vy := radius * omega * math.Cos(theta)
	vz := -0.4 * omega * math.Cos(0.2*theta)

	// Orientación espacial
	yaw := math.Atan2(vy, vx)
	roll := math.Atan(math.Pow(speed, 2) / (9.81 * radius))
	pitch := -0.05

	// Batería LiPo 4S
	const totalFlightSec = 1200.0
	pct := math.Max(0.05, 1.0-(elapsedSec/totalFlightSec))
	voltage := 14.0 + 2.8*pct
	current := 18.0 + 1.5*math.Sin(elapsedSec)
	temp := 25.0 + 15.0*(1.0-pct)

	return &telemetryv1.TelemetryRecord{
		DeviceId:       deviceID,
		SequenceNumber: seq,
		TimestampNs:    timestampNs,
		Position: &telemetryv1.Vector3{
			X: x,
			Y: y,
			Z: z,
		},
		Orientation: eulerToQuaternion(roll, pitch, yaw),
		EulerAngles: &telemetryv1.EulerAngles{
			Roll:  roll,
			Pitch: pitch,
			Yaw:   yaw,
		},
		LinearVelocity: &telemetryv1.Vector3{
			X: vx,
			Y: vy,
			Z: vz,
		},
		AngularVelocity: &telemetryv1.Vector3{
			X: 0.0,
			Y: 0.0,
			Z: omega,
		},
		Armed:        true,
		FlightMode:   telemetryv1.FlightMode_FLIGHT_MODE_OFFBOARD,
		SystemStatus: telemetryv1.SystemStatus_SYSTEM_STATUS_ACTIVE,
		Battery: &telemetryv1.BatteryState{
			VoltageV:     float32(voltage),
			CurrentA:     float32(current),
			Percentage:   float32(pct),
			TemperatureC: float32(temp),
		},
	}
}

// eulerToQuaternion converts Euler angles (roll, pitch, yaw) to Hamilton unit quaternion.
func eulerToQuaternion(roll, pitch, yaw float64) *telemetryv1.Quaternion {
	cy := math.Cos(yaw * 0.5)
	sy := math.Sin(yaw * 0.5)
	cp := math.Cos(pitch * 0.5)
	sp := math.Sin(pitch * 0.5)
	cr := math.Cos(roll * 0.5)
	sr := math.Sin(roll * 0.5)

	return &telemetryv1.Quaternion{
		W: cr*cp*cy + sr*sp*sy,
		X: sr*cp*cy - cr*sp*sy,
		Y: cr*cp*cy + sr*sp*sy,
		Z: cr*cp*sy - sr*sp*cy,
	}
}
