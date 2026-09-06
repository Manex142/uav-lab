package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	telemetryv1 "github.com/Manex142/uav-lab/gen/go/telemetry/v1"
	"google.golang.org/protobuf/proto"
)

// eulerToQuaternion convierte ángulos de Euler (roll, pitch, yaw en radianes)
// a un cuaternión unitario de actitud espacial (Hamilton: w, x, y, z).
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
		Y: cr*sp*cy + sr*cp*sy,
		Z: cr*cp*sy - sr*sp*cy,
	}
}

// computeSyntheticTelemetry genera el estado cinemático y físico del dron
// siguiendo una trayectoria circular suave con variaciones de altura (órbita 3D).
func computeSyntheticTelemetry(
	deviceID string,
	seq uint64,
	elapsedSec float64,
	radius, speed, altitude float64,
) *telemetryv1.TelemetryRecord {
	// Velocidad angular de rotación en el plano horizontal (rad/s): omega = v / R
	omega := speed / radius
	theta := omega * elapsedSec

	// 1. Posición en sistema NED (North-East-Down): Z negativo es altura sobre el suelo
	x := radius * math.Cos(theta)
	y := radius * math.Sin(theta)
	z := -(altitude + 2.0*math.Sin(0.2*theta)) // Oscilación suave de +-2 metros

	// 2. Velocidades lineales (derivada temporal de la posición)
	vx := -radius * omega * math.Sin(theta)
	vy := radius * omega * math.Cos(theta)
	vz := -2.0 * 0.2 * omega * math.Cos(0.2*theta)

	// 3. Orientación:
	// - Guiñada (Yaw): el dron apunta hacia donde se mueve (tangente a la trayectoria)
	yaw := math.Atan2(vy, vx)
	// - Alabeo (Roll): inclinación centrípeta hacia el interior de la curva
	roll := math.Atan(math.Pow(speed, 2) / (9.81 * radius))
	// - Cabeceo (Pitch): ligero morro abajo para mantener avance
	pitch := -0.05

	// 4. Velocidad angular / giroscopio
	angularVel := &telemetryv1.Vector3{
		X: 0.0,
		Y: 0.0,
		Z: omega, // Gira a velocidad angular constante en torno a Z
	}

	// 5. Simulación de batería LiPo 4S (16.8V llena, 14.0V vacía, 20 min de autonomía)
	const totalFlightSec = 1200.0 // 20 minutos
	pctRemaining := math.Max(0.05, 1.0-(elapsedSec/totalFlightSec))
	voltage := 14.0 + 2.8*pctRemaining
	current := 18.5 + 1.2*math.Sin(elapsedSec) // ~18A de consumo de crucero
	cellTemp := 25.0 + 15.0*(1.0-pctRemaining) // Sube progresivamente de 25°C a 40°C

	return &telemetryv1.TelemetryRecord{
		DeviceId:       deviceID,
		SequenceNumber: seq,
		TimestampNs:    time.Now().UnixNano(),
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
		AngularVelocity: angularVel,
		Armed:           true,
		FlightMode:      telemetryv1.FlightMode_FLIGHT_MODE_OFFBOARD,
		SystemStatus:    telemetryv1.SystemStatus_SYSTEM_STATUS_ACTIVE,
		Battery: &telemetryv1.BatteryState{
			VoltageV:     float32(voltage),
			CurrentA:     float32(current),
			Percentage:   float32(pctRemaining),
			TemperatureC: float32(cellTemp),
		},
	}
}

func main() {
	// --- Configuración por Flags de Terminal ---
	deviceID := flag.String("device-id", "uav-alpha-01", "Identificador único del dron")
	targetAddr := flag.String("target-addr", "127.0.0.1:9876", "Dirección UDP destino (host:port)")
	frequencyHz := flag.Int("frequency", 100, "Frecuencia de emisión de paquetes en Hz")
	radius := flag.Float64("radius", 30.0, "Radio de la órbita circular (metros)")
	speed := flag.Float64("speed", 7.0, "Velocidad de crucero horizontal (m/s)")
	altitude := flag.Float64("altitude", 25.0, "Altitud base de vuelo (metros)")
	flag.Parse()

	if *frequencyHz <= 0 {
		log.Fatalf("Frecuencia inválida: %d Hz (debe ser > 0)", *frequencyHz)
	}

	// --- Apertura del Socket UDP ---
	raddr, err := net.ResolveUDPAddr("udp", *targetAddr)
	if err != nil {
		log.Fatalf("Error al resolver dirección UDP %s: %v", *targetAddr, err)
	}

	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		log.Fatalf("Error al conectar socket UDP: %v", err)
	}
	defer conn.Close()

	fmt.Println("================================================================")
	fmt.Println(" 🛸 UAV Synthetic Telemetry Simulator (100 Hz Protobuf -> UDP)")
	fmt.Println("================================================================")
	fmt.Printf(" Device ID    : %s\n", *deviceID)
	fmt.Printf(" Destino UDP  : %s\n", *targetAddr)
	fmt.Printf(" Frecuencia   : %d Hz (intervalo: %v)\n", *frequencyHz, time.Second/time.Duration(*frequencyHz))
	fmt.Printf(" Trayectoria  : Radio=%.1f m, Velocidad=%.1f m/s, Altitud=%.1f m\n", *radius, *speed, *altitude)
	fmt.Println(" Presiona [Ctrl+C] para detener el simulador.")
	fmt.Println("----------------------------------------------------------------")

	// --- Captura de Señales del Sistema Operativo para Parada Limpia (Ctrl+C) ---
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// --- Temporizador de Alta Precisión (Ticker) ---
	interval := time.Second / time.Duration(*frequencyHz)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// --- Variables de Estado y Métricas ---
	var sequenceNumber uint64
	startTime := time.Now()
	lastReportTime := time.Now()
	var packetsSinceReport uint64
	var totalBytesSent uint64

	for {
		select {
		case <-sigChan:
			// El usuario pulsó Ctrl+C: cerramos con resumen
			totalDuration := time.Since(startTime).Seconds()
			avgRate := float64(sequenceNumber) / totalDuration
			fmt.Printf("\n\n⏹️  Deteniendo simulador de forma limpia...\n")
			fmt.Printf("   Paquetes enviados : %d\n", sequenceNumber)
			fmt.Printf("   Bytes transmitidos: %d (%.2f KB)\n", totalBytesSent, float64(totalBytesSent)/1024.0)
			fmt.Printf("   Tiempo de vuelo   : %.2f segundos\n", totalDuration)
			fmt.Printf("   Tasa media real   : %.2f Hz\n", avgRate)
			return

		case now := <-ticker.C:
			sequenceNumber++
			packetsSinceReport++

			elapsedSec := now.Sub(startTime).Seconds()

			// 1. Generar estado físico del dron
			record := computeSyntheticTelemetry(
				*deviceID,
				sequenceNumber,
				elapsedSec,
				*radius,
				*speed,
				*altitude,
			)

			// 2. Serializar a binario Protobuf
			payload, err := proto.Marshal(record)
			if err != nil {
				log.Printf("Error al serializar paquete #%d: %v", sequenceNumber, err)
				continue
			}

			// 3. Emitir datagrama por socket UDP
			n, err := conn.Write(payload)
			if err != nil {
				log.Printf("Error al enviar datagrama UDP: %v", err)
				continue
			}
			totalBytesSent += uint64(n)

			// 4. Cada 1 segundo (o cada N paquetes), mostrar métricas en tiempo real
			if time.Since(lastReportTime) >= time.Second {
				reportDuration := time.Since(lastReportTime).Seconds()
				currentHz := float64(packetsSinceReport) / reportDuration

				fmt.Printf("[%s] seq=%-6d | Pos: (%5.1f, %5.1f, %5.1f)m | Vel: %4.1fm/s | Batt: %4.1f%% (%4.1fV) | Tasa: %5.1f Hz | Payload: %d B\n",
					*deviceID,
					sequenceNumber,
					record.GetPosition().GetX(),
					record.GetPosition().GetY(),
					record.GetPosition().GetZ(),
					math.Sqrt(record.GetLinearVelocity().GetX()*record.GetLinearVelocity().GetX()+record.GetLinearVelocity().GetY()*record.GetLinearVelocity().GetY()),
					record.GetBattery().GetPercentage()*100.0,
					record.GetBattery().GetVoltageV(),
					currentHz,
					len(payload),
				)

				lastReportTime = time.Now()
				packetsSinceReport = 0
			}
		}
	}
}
