package tui

import (
	"sort"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/Manex142/uav-lab/internal/ingest"
	"github.com/Manex142/uav-lab/internal/ledger"
)

type tickMsg time.Time

// EventItem represents an event emitted by the fleet watchdog.
type EventItem struct {
	Time     time.Time
	Type     string // "ONLINE", "LOST", "RESTORED"
	DeviceID string
	Text     string
}

type Model struct {
	registry  *ledger.DeviceRegistry
	metrics   *ingest.Metrics
	eventChan <-chan EventItem

	// Viewport & Terminal sizing
	width  int
	height int

	// Runtime metrics tracking
	startTime   time.Time
	lastTick    time.Time
	prevPackets uint64
	prevBytes   uint64
	currentHz   float64
	currentKBps float64
	hzHistory   []float64

	// Fleet state & navigation
	paused        bool
	devices       []ledger.DeviceState
	cursorIndex   int
	sortByBattery bool
	events        []EventItem
	snap          ingest.Snapshot
}

func NewModel(registry *ledger.DeviceRegistry, metrics *ingest.Metrics, eventChan <-chan EventItem) Model {
	return Model{
		registry:    registry,
		metrics:     metrics,
		eventChan:   eventChan,
		startTime:   time.Now(),
		lastTick:    time.Now(),
		devices:     make([]ledger.DeviceState, 0),
		events:      make([]EventItem, 0),
		hzHistory:   make([]float64, 0, 30),
		width:       100,
		height:      35,
		cursorIndex: 0,
	}
}

func (m Model) Init() tea.Cmd {
	return tickCmd()
}

func tickCmd() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case " ", "space":
			m.paused = !m.paused
			return m, nil
		case "up", "k":
			if m.cursorIndex > 0 {
				m.cursorIndex--
			}
			return m, nil
		case "down", "j":
			if m.cursorIndex < len(m.devices)-1 {
				m.cursorIndex++
			}
			return m, nil
		case "b":
			m.sortByBattery = !m.sortByBattery
			m.cursorIndex = 0
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		// 1. Drain pending watchdog events from channel
		if m.eventChan != nil {
		drainEvents:
			for {
				select {
				case evt := <-m.eventChan:
					m.events = append(m.events, evt)
					if len(m.events) > 15 {
						m.events = m.events[1:]
					}
				default:
					break drainEvents
				}
			}
		}

		// 2. Sample metrics if not paused
		if !m.paused {
			now := time.Time(msg)
			delta := now.Sub(m.lastTick).Seconds()
			if delta > 0 && m.metrics != nil {
				m.snap = m.metrics.GetSnapshot()
				m.currentHz = float64(m.snap.PacketsReceived-m.prevPackets) / delta
				m.currentKBps = (float64(m.snap.BytesReceived-m.prevBytes) / 1024.0) / delta
				m.prevPackets = m.snap.PacketsReceived
				m.prevBytes = m.snap.BytesReceived

				// Track sparkline history (last 24 points)
				m.hzHistory = append(m.hzHistory, m.currentHz)
				if len(m.hzHistory) > 24 {
					m.hzHistory = m.hzHistory[1:]
				}
			}
			m.lastTick = now

			// 3. Update device fleet
			if m.registry != nil {
				devs := m.registry.GetAll()
				if m.sortByBattery {
					sort.Slice(devs, func(i, j int) bool {
						var batI, batJ float32 = 1.0, 1.0
						if devs[i].Battery != nil {
							batI = devs[i].Battery.GetPercentage()
						}
						if devs[j].Battery != nil {
							batJ = devs[j].Battery.GetPercentage()
						}
						return batI < batJ
					})
				} else {
					sort.Slice(devs, func(i, j int) bool {
						return devs[i].DeviceID < devs[j].DeviceID
					})
				}
				m.devices = devs

				// Clamp cursor index if fleet size changed
				if m.cursorIndex >= len(m.devices) && len(m.devices) > 0 {
					m.cursorIndex = len(m.devices) - 1
				}
			}
		}
		return m, tickCmd()
	}

	return m, nil
}
