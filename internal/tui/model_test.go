package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/Manex142/uav-lab/internal/ingest"
	"github.com/Manex142/uav-lab/internal/ledger"
)

func TestTUIModelLifecycle(t *testing.T) {
	metrics := &ingest.Metrics{}
	registry := ledger.NewDeviceRegistry()

	m := NewModel(registry, metrics, nil)

	// 1. Verify Init returns a tick command
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("expected Init() to return a non-nil tick command")
	}

	// 2. Test Window Resize
	updatedModel, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updatedModel.(Model)
	if m.width != 120 || m.height != 40 {
		t.Fatalf("expected dimensions 120x40, got %dx%d", m.width, m.height)
	}

	// 3. Test Pause Toggle
	updatedModel, _ = m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	m = updatedModel.(Model)
	if !m.paused {
		t.Fatal("expected model to be paused after space keypress")
	}

	// 4. Test Quit
	_, quitCmd := m.Update(tea.KeyPressMsg{Text: "q"})
	if quitCmd == nil {
		t.Fatal("expected quitCmd on 'q' keypress")
	}

	// 5. Test View Rendering
	view := m.View()
	if !view.AltScreen {
		t.Fatal("expected AltScreen to be true in View")
	}
	if !strings.Contains(view.Content, "UAV LAB // REAL-TIME TELEMETRY MONITOR") {
		t.Fatalf("expected view content to contain header title, got: %s", view.Content)
	}
}
