package ingest_test

import (
	"fmt"
	"net"
	"sync/atomic"
	"testing"
	"time"

	telemetryv1 "github.com/Manex142/uav-lab/gen/go/telemetry/v1"
	"github.com/Manex142/uav-lab/internal/ingest"
	"google.golang.org/protobuf/proto"
)

func TestIngestionPipeline_EndToEnd(t *testing.T) {
	metrics := &ingest.Metrics{}

	// Encontrar un puerto UDP libre
	tempConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("no se pudo abrir puerto libre: %v", err)
	}
	port := tempConn.LocalAddr().(*net.UDPAddr).Port
	tempConn.Close()

	addr := fmt.Sprintf("127.0.0.1:%d", port)

	// Inicializar Listener y WorkerPool
	listener := ingest.NewListener(addr, 1024*1024, 1000, metrics)
	if err := listener.Start(); err != nil {
		t.Fatalf("falló listener.Start(): %v", err)
	}
	defer listener.Stop()

	var processedCount uint64
	handler := func(record *telemetryv1.TelemetryRecord) {
		atomic.AddUint64(&processedCount, 1)
	}

	workers := ingest.NewWorkerPool(4, listener.PacketChan(), metrics, handler)
	workers.Start()
	defer workers.Stop()

	// Enviar 200 paquetes de prueba por UDP
	raddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		t.Fatalf("error resolviendo raddr: %v", err)
	}
	senderConn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		t.Fatalf("error conectando sender: %v", err)
	}
	defer senderConn.Close()

	const totalPackets = 200
	for seq := uint64(1); seq <= totalPackets; seq++ {
		record := &telemetryv1.TelemetryRecord{
			DeviceId:       "test-uav",
			SequenceNumber: seq,
			TimestampNs:    time.Now().UnixNano(),
			Position:       &telemetryv1.Vector3{X: 10, Y: 20, Z: -30},
			Armed:          true,
		}
		data, err := proto.Marshal(record)
		if err != nil {
			t.Fatalf("error serializando: %v", err)
		}
		if _, err := senderConn.Write(data); err != nil {
			t.Fatalf("error enviando: %v", err)
		}
		time.Sleep(1 * time.Millisecond)
	}

	// Esperar brevemente a que los workers vacíen el canal
	time.Sleep(100 * time.Millisecond)

	snap := metrics.GetSnapshot()
	t.Logf("Snapshot: %+v", snap)
	t.Logf("Processed by handler: %d", atomic.LoadUint64(&processedCount))

	if snap.PacketsReceived != totalPackets {
		t.Errorf("esperados %d paquetes, recibidos %d", totalPackets, snap.PacketsReceived)
	}
	if snap.DecodeErrors != 0 {
		t.Errorf("esperados 0 errores de decodificación, obtenidos %d", snap.DecodeErrors)
	}
	if snap.PacketsDropped != 0 {
		t.Errorf("esperados 0 paquetes descartados, obtenidos %d", snap.PacketsDropped)
	}
	if workers.GetTotalPacketLoss() != 0 {
		t.Errorf("esperados 0 paquetes perdidos en red, obtenidos %d", workers.GetTotalPacketLoss())
	}
}
