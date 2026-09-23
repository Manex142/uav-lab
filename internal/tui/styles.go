package tui

import (
	"charm.land/lipgloss/v2"
)

var (
	// Color palette
	primaryColor   = lipgloss.Color("#7D56F4") // Purple/Violet
	secondaryColor = lipgloss.Color("#00D2FF") // Cyan
	successColor   = lipgloss.Color("#04B575") // Emerald Green
	warningColor   = lipgloss.Color("#FFB000") // Amber
	dangerColor    = lipgloss.Color("#FF3B30") // Coral Red
	subtleColor    = lipgloss.Color("#6C6C75") // Muted Gray
	bgColor        = lipgloss.Color("#1A1B26") // Dark Tokyo Night

	// Header Styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(primaryColor).
			Padding(0, 1)

	headerBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(0, 1)

	// Panel Styles
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#3B4261")).
			Padding(0, 1)

	activePanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(secondaryColor).
				Padding(0, 1)

	panelTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(secondaryColor)

	// Status Badges
	onlineBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(successColor).
			Padding(0, 1).
			SetString("ONLINE")

	offlineBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(dangerColor).
			Padding(0, 1).
			SetString("LOST")

	// Metric Values
	metricLabelStyle = lipgloss.NewStyle().
				Foreground(subtleColor)

	metricValStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#E0AF68"))

	// Footer / Help
	keyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor)

	helpStyle = lipgloss.NewStyle().
			Foreground(subtleColor)
)
