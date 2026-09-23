package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	telemetryv1 "github.com/Manex142/uav-lab/gen/go/telemetry/v1"
	"github.com/Manex142/uav-lab/internal/ledger"
)

func (m Model) View() tea.View {
	content := m.renderContent()
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m Model) renderContent() string {
	var b strings.Builder

	// Calculate widths
	totalWidth := m.width
	if totalWidth < 90 {
		totalWidth = 90
	}
	fleetWidth := totalWidth - 44
	if fleetWidth < 54 {
		fleetWidth = 54
	}
	metricsWidth := totalWidth - fleetWidth - 4

	// --- 1. Top Header ---
	b.WriteString(m.renderHeader(totalWidth))
	b.WriteString("\n\n")

	// --- 2. Middle Row: Fleet Table & Ingestion Metrics ---
	fleetPanel := m.renderFleetPanel(fleetWidth)
	metricsPanel := m.renderMetricsPanel(metricsWidth)
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, fleetPanel, "  ", metricsPanel))
	b.WriteString("\n\n")

	// --- 3. Bottom Row: Drone Inspector & Watchdog Event Stream ---
	inspectorWidth := fleetWidth
	eventsWidth := metricsWidth
	inspectorPanel := m.renderInspectorPanel(inspectorWidth)
	eventsPanel := m.renderEventsPanel(eventsWidth)
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, inspectorPanel, "  ", eventsPanel))
	b.WriteString("\n\n")

	// --- 4. Footer / Keybindings ---
	b.WriteString(m.renderFooter())

	return b.String()
}

func (m Model) renderHeader(totalWidth int) string {
	title := titleStyle.Render("🛸 UAV LAB // REAL-TIME TELEMETRY MONITOR")

	activeCount := 0
	for _, d := range m.devices {
		if d.Status == ledger.StatusOnline {
			activeCount++
		}
	}

	statusPill := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(successColor).
		Padding(0, 1).
		Render("● GATEWAY ACTIVE")

	fleetBadge := lipgloss.NewStyle().
		Bold(true).
		Foreground(secondaryColor).
		Render(fmt.Sprintf("🛸 FLEET: %d ACTIVE / %d TOTAL", activeCount, len(m.devices)))

	hzBadge := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#E0AF68")).
		Render(fmt.Sprintf("⚡ %5.1f PKT/S", m.currentHz))

	latBadge := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7AA2F7")).
		Render(fmt.Sprintf("⏱ %5.2f MS LAT", m.snap.AvgLatencyMs))

	sortBadge := ""
	if m.sortByBattery {
		sortBadge = "   " + lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#1A1B26")).Background(warningColor).Padding(0, 1).Render("SORT: LOW BATTERY")
	}

	pauseBadge := ""
	if m.paused {
		pauseBadge = "   " + lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(dangerColor).Padding(0, 1).Render("PAUSED")
	}

	headerContent := fmt.Sprintf("%s   %s   %s   %s%s%s", statusPill, fleetBadge, hzBadge, latBadge, sortBadge, pauseBadge)

	return headerBoxStyle.Width(totalWidth - 4).Render(
		lipgloss.JoinVertical(lipgloss.Left, title, "", headerContent),
	)
}

func (m Model) renderFleetPanel(width int) string {
	var rows strings.Builder

	// Table Header
	header := fmt.Sprintf("  %-9s %-9s %-9s %-12s %-9s %-9s",
		"DEVICE ID", "STATUS", "ALTITUDE", "BATTERY", "SPEED", "LAST SEEN")
	rows.WriteString(panelTitleStyle.Render(header))
	rows.WriteString("\n" + strings.Repeat("─", width-4) + "\n")

	if len(m.devices) == 0 {
		rows.WriteString("\n" + lipgloss.NewStyle().Faint(true).Render("  Esperando telemetría de drones vía UDP (puerto 9876)...") + "\n\n")
	} else {
		// Windowed scrolling for large fleets (show up to 10 rows around cursor)
		maxVisible := 10
		start := 0
		if m.cursorIndex >= maxVisible {
			start = m.cursorIndex - maxVisible + 1
		}
		end := start + maxVisible
		if end > len(m.devices) {
			end = len(m.devices)
		}

		for i := start; i < end; i++ {
			dev := m.devices[i]
			isCursor := i == m.cursorIndex

			cursorPrefix := "  "
			rowStyle := lipgloss.NewStyle()
			if isCursor {
				cursorPrefix = lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Render("▶ ")
				rowStyle = rowStyle.Bold(true).Foreground(lipgloss.Color("#FFFFFF"))
			}

			statusStr := onlineBadge.Render()
			if dev.Status != ledger.StatusOnline {
				statusStr = offlineBadge.Render()
			}

			alt := 0.0
			if dev.Position != nil {
				alt = math.Abs(dev.Position.GetZ())
			}

			speed := 0.0
			if dev.LinearVelocity != nil {
				speed = getSpeed(dev.LinearVelocity)
			}

			batPct := float32(100.0)
			if dev.Battery != nil {
				batPct = dev.Battery.GetPercentage()
				if batPct <= 1.0 {
					batPct *= 100.0
				}
			}
			batStr := renderBatteryBar(batPct)

			lastSeenAgo := time.Since(dev.LastSeen)
			lastSeenStr := fmt.Sprintf("%3.0fms ago", float64(lastSeenAgo.Milliseconds()))
			if lastSeenAgo > time.Second {
				lastSeenStr = fmt.Sprintf("%3.1fs ago", lastSeenAgo.Seconds())
			}

			row := fmt.Sprintf("%s%-9s %-9s %5.1f m   %-12s %4.1f m/s  %-9s",
				cursorPrefix, dev.DeviceID, statusStr, alt, batStr, speed, lastSeenStr)
			rows.WriteString(rowStyle.Render(row) + "\n")
		}

		if len(m.devices) > maxVisible {
			rows.WriteString(lipgloss.NewStyle().Faint(true).Render(
				fmt.Sprintf("\n  [Dron %d de %d] Usa ↑/↓ para navegar la lista completa", m.cursorIndex+1, len(m.devices)),
			))
		}
	}

	return panelStyle.Width(width).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Render("◆ SWARM FLEET TELEMETRY"),
			"",
			rows.String(),
		),
	)
}

func (m Model) renderMetricsPanel(width int) string {
	uptime := time.Since(m.startTime).Truncate(time.Second)
	totalMB := float64(m.snap.BytesReceived) / (1024.0 * 1024.0)

	sparklineWidth := width - 12
	if sparklineWidth > 24 {
		sparklineWidth = 24
	}
	sparklineStr := renderSparkline(m.hzHistory, sparklineWidth)

	lines := []string{
		lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Render("◆ INGESTION METRICS"),
		"",
		fmt.Sprintf("%s %s", metricLabelStyle.Render("Throughput :"), metricValStyle.Render(fmt.Sprintf("%7.1f Hz", m.currentHz))),
		fmt.Sprintf("%s %s", metricLabelStyle.Render("Throughput :"), lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Render(sparklineStr)),
		fmt.Sprintf("%s %s", metricLabelStyle.Render("Bandwidth  :"), metricValStyle.Render(fmt.Sprintf("%7.2f KB/s", m.currentKBps))),
		fmt.Sprintf("%s %s", metricLabelStyle.Render("Total Pkts :"), metricValStyle.Render(fmt.Sprintf("%d", m.snap.PacketsReceived))),
		fmt.Sprintf("%s %s", metricLabelStyle.Render("Total Ingest:"), metricValStyle.Render(fmt.Sprintf("%.2f MB", totalMB))),
		fmt.Sprintf("%s %s", metricLabelStyle.Render("Avg Latency:"), metricValStyle.Render(fmt.Sprintf("%.3f ms", m.snap.AvgLatencyMs))),
		"",
		fmt.Sprintf("%s %s", metricLabelStyle.Render("Dropped RAM:"), renderAlertMetric(m.snap.PacketsDropped)),
		fmt.Sprintf("%s %s", metricLabelStyle.Render("Decode Errs:"), renderAlertMetric(m.snap.DecodeErrors)),
		fmt.Sprintf("%s %s", metricLabelStyle.Render("Uptime     :"), metricValStyle.Render(uptime.String())),
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m Model) renderInspectorPanel(width int) string {
	title := lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Render("◆ TELEMETRY INSPECTOR")

	if len(m.devices) == 0 || m.cursorIndex >= len(m.devices) {
		return panelStyle.Width(width).Render(
			lipgloss.JoinVertical(lipgloss.Left, title, "", lipgloss.NewStyle().Faint(true).Render("  Ningún dron seleccionado")),
		)
	}

	dev := m.devices[m.cursorIndex]
	header := fmt.Sprintf("%s  [Status: %s | Seq: #%d]",
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Render(dev.DeviceID),
		dev.Status,
		dev.SequenceNumber,
	)

	posX, posY, posZ := 0.0, 0.0, 0.0
	if dev.Position != nil {
		posX, posY, posZ = dev.Position.GetX(), dev.Position.GetY(), math.Abs(dev.Position.GetZ())
	}
	posLine := fmt.Sprintf("%s X: %6.1fm  Y: %6.1fm  Z(Alt): %6.1fm",
		metricLabelStyle.Render("Posición 3D :"), posX, posY, posZ)

	roll, pitch, yaw := 0.0, 0.0, 0.0
	if dev.EulerAngles != nil {
		roll = dev.EulerAngles.GetRoll() * (180.0 / math.Pi)
		pitch = dev.EulerAngles.GetPitch() * (180.0 / math.Pi)
		yaw = dev.EulerAngles.GetYaw() * (180.0 / math.Pi)
	}
	rpyLine := fmt.Sprintf("%s Roll: %5.1f°  Pitch: %5.1f°  Yaw: %5.1f°",
		metricLabelStyle.Render("Orientación :"), roll, pitch, yaw)

	volt, curr, temp := 0.0, 0.0, 0.0
	if dev.Battery != nil {
		volt = float64(dev.Battery.GetVoltageV())
		curr = float64(dev.Battery.GetCurrentA())
		temp = float64(dev.Battery.GetTemperatureC())
	}
	batLine := fmt.Sprintf("%s %.1fV  │  %.1fA  │  Temp: %.1f°C",
		metricLabelStyle.Render("Batería LiPo:"), volt, curr, temp)

	armedStr := "DISARMED"
	if dev.Armed {
		armedStr = lipgloss.NewStyle().Bold(true).Foreground(successColor).Render("ARMED")
	}
	modeLine := fmt.Sprintf("%s %s  │  Modo: %s",
		metricLabelStyle.Render("Estado Vuelo:"), armedStr, dev.FlightMode.String())

	return panelStyle.Width(width).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			title,
			header,
			strings.Repeat("─", width-4),
			posLine,
			rpyLine,
			batLine,
			modeLine,
		),
	)
}

func (m Model) renderEventsPanel(width int) string {
	title := lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Render("◆ FLEET EVENTS (WATCHDOG)")

	var lines []string
	lines = append(lines, title, strings.Repeat("─", width-4))

	if len(m.events) == 0 {
		lines = append(lines, lipgloss.NewStyle().Faint(true).Render("  Sin eventos recientes de conexión"))
	} else {
		// Show up to 4 most recent events
		start := 0
		if len(m.events) > 4 {
			start = len(m.events) - 4
		}
		for _, evt := range m.events[start:] {
			timeStr := evt.Time.Format("15:04:05")
			color := successColor
			if evt.Type == "LOST" {
				color = dangerColor
			} else if evt.Type == "RESTORED" {
				color = successColor
			} else if evt.Type == "LOSS" {
				color = warningColor
			}

			bullet := lipgloss.NewStyle().Bold(true).Foreground(color).Render("●")
			tag := lipgloss.NewStyle().Bold(true).Foreground(color).Render(evt.DeviceID)
			lines = append(lines, fmt.Sprintf("[%s] %s %s %s", timeStr, bullet, tag, evt.Text))
		}
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m Model) renderFooter() string {
	qKey := keyStyle.Render("[q]")
	navKey := keyStyle.Render("[↑/↓]")
	batKey := keyStyle.Render("[b]")
	spaceKey := keyStyle.Render("[space]")

	helpText := fmt.Sprintf("%s Salir  │  %s Seleccionar Dron  │  %s Ordenar Batería  │  %s Pausar",
		qKey, navKey, batKey, spaceKey)

	return helpStyle.Render(" " + helpText)
}

func renderSparkline(history []float64, width int) string {
	if len(history) == 0 {
		return strings.Repeat(" ", width)
	}
	bars := []rune{' ', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	maxVal := 1.0
	for _, v := range history {
		if v > maxVal {
			maxVal = v
		}
	}

	var sb strings.Builder
	start := 0
	if len(history) > width {
		start = len(history) - width
	}
	for _, v := range history[start:] {
		idx := int((v / maxVal) * float64(len(bars)-1))
		if idx < 0 {
			idx = 0
		} else if idx >= len(bars) {
			idx = len(bars) - 1
		}
		sb.WriteRune(bars[idx])
	}

	// Pad with spaces if less than width
	pad := width - sb.Len()
	if pad > 0 {
		return strings.Repeat(" ", pad) + sb.String()
	}
	return sb.String()
}

func renderBatteryBar(pct float32) string {
	if pct <= 0 {
		return "[░░░░]   0%"
	}
	blocks := int((pct / 100.0) * 4)
	if blocks > 4 {
		blocks = 4
	}
	bar := strings.Repeat("█", blocks) + strings.Repeat("░", 4-blocks)

	style := successColor
	if pct < 25 {
		style = dangerColor
	} else if pct < 50 {
		style = warningColor
	}

	return lipgloss.NewStyle().Foreground(style).Render(fmt.Sprintf("[%s] %2.0f%%", bar, pct))
}

func renderAlertMetric(count uint64) string {
	if count == 0 {
		return lipgloss.NewStyle().Foreground(successColor).Render("0 (clean)")
	}
	return lipgloss.NewStyle().Foreground(dangerColor).Bold(true).Render(fmt.Sprintf("%d ⚠️", count))
}

func getSpeed(v *telemetryv1.Vector3) float64 {
	if v == nil {
		return 0
	}
	return math.Sqrt(v.GetX()*v.GetX() + v.GetY()*v.GetY() + v.GetZ()*v.GetZ())
}
