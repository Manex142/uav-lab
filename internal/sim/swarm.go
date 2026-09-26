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
	DroneCount   int     // Total number of concurrent UAV simulators
	TargetAddr   string  // Destination UDP address (e.g. 127.0.0.1:9876)
	FrequencyHz  int     // Telemetry frequency per drone in Hz (e.g. 100)
	EnableJitter bool    // Stagger initial emissions to avoid micro-bursting
	MinRadius    float64 // Minimum orbit radius (meters)
	MaxRadius    float64 // Maximum orbit radius (meters)
	MinSpeed     float64 // Minimum horizontal cruise speed (m/s)
	MaxSpeed     float64 // Maximum horizontal cruise speed (m/s)
	BaseAltitude float64 // Base flight altitude (meters)
}

// DefaultSwarmConfig returns production-tested defaults for load testing.
func DefaultSwarmConfig() SwarmConfig {
	return SwarmConfig{
		DroneCount:   10,
		TargetAddr:   "127.0.0.1:9876",
		FrequencyHz:  100,
		EnableJitter: true,
		MinRadius:    80.0,
		MaxRadius:    220.0,
		MinSpeed:     8.0,
		MaxSpeed:     15.0,
		BaseAltitude: 35.0,
	}
}

// PatrolSector defines a geographic operational surveillance zone in local Cartesian NED meters.
type PatrolSector struct {
	Name        string
	CenterNorth float64 // meters North (+X) from home datum
	CenterEast  float64 // meters East (+Y) from home datum
}

// TacticalSectors defines surveillance zones across San Sebastián / Donostia (relative to Plaza Gipuzkoa).
var TacticalSectors = []PatrolSector{
	{Name: "La Concha Bay Promenade", CenterNorth: 80.0, CenterEast: -450.0},
	{Name: "Zurriola Beach & Kursaal", CenterNorth: 260.0, CenterEast: 380.0},
	{Name: "Urumea River & Bridges", CenterNorth: -180.0, CenterEast: 240.0},
	{Name: "Mount Urgull & Castle", CenterNorth: 540.0, CenterEast: -120.0},
	{Name: "Old Town & Boulevard", CenterNorth: 160.0, CenterEast: -90.0},
	{Name: "Buen Pastor Cathedral", CenterNorth: -360.0, CenterEast: -50.0},
	{Name: "Gros District", CenterNorth: -60.0, CenterEast: 540.0},
	{Name: "Bahía de la Concha Outer", CenterNorth: 200.0, CenterEast: -720.0},
	{Name: "Paseo Nuevo Coastal Cliff", CenterNorth: 640.0, CenterEast: 140.0},
	{Name: "Amara Valley Corridor", CenterNorth: -620.0, CenterEast: 180.0},
}

// DroneTrajectory specifies physical flight path dynamics for an autonomous UAV.
type DroneTrajectory struct {
	DeviceID      string
	CenterNorth   float64
	CenterEast    float64
	RadiusNorth   float64
	RadiusEast    float64
	Speed         float64
	Altitude      float64
	PhaseOffset   float64
	Direction     float64 // +1.0 clockwise, -1.0 counter-clockwise
	InitialBatPct float64
}

// SwarmMetrics tracks aggregated real-time transmission statistics.
type SwarmMetrics struct {
	PacketsSent   atomic.Uint64
	BytesSent     atomic.Uint64
	NetworkErrors atomic.Uint64
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

		// Assign a distinct tactical surveillance sector
		sector := TacticalSectors[(i-1)%len(TacticalSectors)]

		// Sector dispersion for larger swarms
		sectorDispersionX := 0.0
		sectorDispersionY := 0.0
		if s.cfg.DroneCount > len(TacticalSectors) {
			sectorDispersionX = (rng.Float64() - 0.5) * 80.0
			sectorDispersionY = (rng.Float64() - 0.5) * 80.0
		}

		centerNorth := sector.CenterNorth + sectorDispersionX
		centerEast := sector.CenterEast + sectorDispersionY

		// Randomize drone flight dynamics
		baseRadius := s.cfg.MinRadius + rng.Float64()*(s.cfg.MaxRadius-s.cfg.MinRadius)
		radiusNorth := baseRadius * (0.85 + 0.3*rng.Float64())
		radiusEast := baseRadius * (0.85 + 0.3*rng.Float64())

		speed := s.cfg.MinSpeed + rng.Float64()*(s.cfg.MaxSpeed-s.cfg.MinSpeed)
		alt := s.cfg.BaseAltitude + (float64(i%6) * 8.0) + rng.Float64()*4.0 // Staggered altitude corridors (35m - 85m)

		// Initial phase offset to spread drones uniformly along their orbits
		phaseOffset := (float64(i) / float64(s.cfg.DroneCount)) * 2.0 * math.Pi

		// Alternate orbit direction
		direction := 1.0
		if i%2 == 1 {
			direction = -1.0
		}

		// Varied initial battery percentage (with simulated low battery warning assets)
		initialBatPct := 0.95 - (float64((i*19)%60) / 100.0)
		if i == 5 {
			initialBatPct = 0.17 // Low battery alert asset
		} else if i == 9 {
			initialBatPct = 0.28 // Warning battery tier
		}

		// Compute initial phase jitter (spread evenly across 1 interval period)
		var jitter time.Duration
		if s.cfg.EnableJitter && s.cfg.DroneCount > 1 {
			jitter = time.Duration(float64(interval) * (float64(i) / float64(s.cfg.DroneCount)))
		}

		traj := DroneTrajectory{
			DeviceID:      deviceID,
			CenterNorth:   centerNorth,
			CenterEast:    centerEast,
			RadiusNorth:   radiusNorth,
			RadiusEast:    radiusEast,
			Speed:         speed,
			Altitude:      alt,
			PhaseOffset:   phaseOffset,
			Direction:     direction,
			InitialBatPct: initialBatPct,
		}

		s.wg.Add(1)
		go s.runDrone(traj, raddr, interval, jitter)
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
	traj DroneTrajectory,
	raddr *net.UDPAddr,
	interval time.Duration,
	jitter time.Duration,
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

			record := computeSyntheticRecord(traj, seq, now.UnixNano(), elapsedSec)

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

// computeSyntheticRecord generates physical kinematics for a 3D orbital patrol trajectory.
func computeSyntheticRecord(
	traj DroneTrajectory,
	seq uint64,
	timestampNs int64,
	elapsedSec float64,
) *telemetryv1.TelemetryRecord {
	meanRadius := math.Sqrt(0.5 * (traj.RadiusNorth*traj.RadiusNorth + traj.RadiusEast*traj.RadiusEast))
	if meanRadius < 5.0 {
		meanRadius = 5.0
	}

	omega := traj.Direction * (traj.Speed / meanRadius)
	theta := omega*elapsedSec + traj.PhaseOffset

	// Local Cartesian NED position (Z negative = altitude above ground)
	x := traj.CenterNorth + traj.RadiusNorth*math.Cos(theta)
	y := traj.CenterEast + traj.RadiusEast*math.Sin(theta)
	z := -(traj.Altitude + 2.5*math.Sin(0.15*theta))

	// Linear velocities (NED)
	vx := -traj.RadiusNorth * omega * math.Sin(theta)
	vy := traj.RadiusEast * omega * math.Cos(theta)
	vz := -0.15 * 2.5 * omega * math.Cos(0.15*theta)

	// Spatial orientation: yaw points towards velocity vector in NED
	yaw := math.Atan2(vy, vx)
	currentSpeed := math.Hypot(vx, vy)
	roll := math.Atan(math.Pow(currentSpeed, 2)/(9.81*meanRadius)) * traj.Direction
	pitch := -0.04 // Gentle forward pitch during cruise

	// LiPo battery discharge simulation
	const totalFlightSec = 1800.0 // 30 minutes nominal endurance
	pct := math.Max(0.04, traj.InitialBatPct-(elapsedSec/totalFlightSec))
	voltage := 14.0 + 2.8*pct
	current := 16.0 + 2.0*math.Sin(elapsedSec*0.5)
	temp := 26.0 + 14.0*(1.0-pct)

	return &telemetryv1.TelemetryRecord{
		DeviceId:       traj.DeviceID,
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
