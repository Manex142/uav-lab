package ingest

import (
	"fmt"
	"log"
	"net"
	"sync"
)

// DefaultSocketBufferSize es el tamaño del buffer del kernel del SO (4 MB).
const DefaultSocketBufferSize = 4 * 1024 * 1024

// DefaultQueueCapacity es el número de paquetes que caben en el canal en RAM (10.000).
const DefaultQueueCapacity = 10000

// Listener encapsula el socket UDP de alto rendimiento y el bucle de ingestión no bloqueante.
type Listener struct {
	listenAddr       string
	socketBufferSize int
	queueCapacity    int
	metrics          *Metrics

	conn       *net.UDPConn
	packetChan chan []byte
	quitChan   chan struct{}
	wg         sync.WaitGroup
}

// NewListener crea una instancia del listener con socket UDP y canal bufferizado.
func NewListener(
	listenAddr string,
	socketBufferSize int,
	queueCapacity int,
	metrics *Metrics,
) *Listener {
	if socketBufferSize <= 0 {
		socketBufferSize = DefaultSocketBufferSize
	}
	if queueCapacity <= 0 {
		queueCapacity = DefaultQueueCapacity
	}

	return &Listener{
		listenAddr:       listenAddr,
		socketBufferSize: socketBufferSize,
		queueCapacity:    queueCapacity,
		metrics:          metrics,
		packetChan:       make(chan []byte, queueCapacity),
		quitChan:         make(chan struct{}),
	}
}

// PacketChan devuelve el canal de solo lectura para que lo consuman los trabajadores.
func (l *Listener) PacketChan() <-chan []byte {
	return l.packetChan
}

// Start abre el socket UDP, configura el buffer de Linux y arranca el loop de lectura.
func (l *Listener) Start() error {
	addr, err := net.ResolveUDPAddr("udp", l.listenAddr)
	if err != nil {
		return fmt.Errorf("error resolviendo dirección UDP %s: %w", l.listenAddr, err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("error abriendo socket net.ListenUDP en %s: %w", l.listenAddr, err)
	}
	l.conn = conn

	// Configurar el buffer de lectura en el Kernel de Linux (SO_RCVBUF)
	if err := l.conn.SetReadBuffer(l.socketBufferSize); err != nil {
		log.Printf("[Listener] ⚠️ Advertencia fijando SetReadBuffer a %d bytes: %v", l.socketBufferSize, err)
	} else {
		log.Printf("[Listener] Buffer del socket kernel fijado a %d bytes (%.1f MB)",
			l.socketBufferSize, float64(l.socketBufferSize)/(1024*1024))
	}

	l.wg.Add(1)
	go l.readLoop()

	return nil
}

// Stop cierra el socket y espera a que el bucle de lectura finalice.
func (l *Listener) Stop() {
	close(l.quitChan)
	if l.conn != nil {
		l.conn.Close()
	}
	l.wg.Wait()
	close(l.packetChan)
}

// readLoop es el bucle ultrarrápido que extrae datagramas del SO y los deposita en el canal.
func (l *Listener) readLoop() {
	defer l.wg.Done()

	// Buffer temporal de lectura (2048 bytes es suficiente para cualquier datagrama que respete MTU)
	buf := make([]byte, 2048)

	for {
		select {
		case <-l.quitChan:
			return
		default:
		}

		n, _, err := l.conn.ReadFromUDP(buf)
		if err != nil {
			select {
			case <-l.quitChan:
				// Cierre limpio esperado
				return
			default:
				log.Printf("[Listener] Error leyendo socket UDP: %v", err)
				continue
			}
		}

		if n <= 0 {
			continue
		}

		// Copiar el fragmento de bytes exacto del paquete recibido
		packet := make([]byte, n)
		copy(packet, buf[:n])

		// Desacoplamiento no bloqueante:
		// Si el canal en RAM está lleno (los workers van lentos), se descarta el paquete
		// para no congelar la tarjeta de red del sistema operativo.
		select {
		case l.packetChan <- packet:
			// Encolado con éxito en RAM
		default:
			l.metrics.IncPacketsDropped()
			log.Printf("[Listener] ⚠️ Buffer en RAM lleno (%d en cola), paquete descartado!", len(l.packetChan))
		}
	}
}
