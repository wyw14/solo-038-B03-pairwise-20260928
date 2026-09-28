package register

import (
	"testing"
	"time"

	"github.com/wyw14/solo-038/internal/sensor"
)

func frameAt(id string, meter float64, receivedAt time.Time) sensor.CameraFrame {
	return sensor.CameraFrame{ID: id, Camera: "register-camera", JobGeneration: 1, PlateGeneration: 1, WebMeter: meter, CapturedAt: receivedAt, ReceivedAt: receivedAt}
}

func TestAddRejectsOutOfOrderFrame(t *testing.T) {
	base := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	state := NewState()
	if err := state.Start(1, 1, 0, base); err != nil {
		t.Fatalf("start: %v", err)
	}
	// A later frame establishes the window's time progress first.
	if err := state.Add(frameAt("late-arrival", 1, base.Add(200*time.Millisecond))); err != nil {
		t.Fatalf("add in-order frame: %v", err)
	}
	// An earlier-timestamped frame arrives afterwards at the same meter.
	if err := state.Add(frameAt("out-of-order", 1, base.Add(100*time.Millisecond))); err == nil {
		t.Fatalf("expected out-of-order frame to be rejected")
	}
	window := state.Snapshot()
	if window.Samples != 1 {
		t.Fatalf("late frame must not extend stable window: samples = %d, want 1", window.Samples)
	}
	if !window.LastAt.Equal(base.Add(200 * time.Millisecond)) {
		t.Fatalf("late frame must not move window time backwards: last_at = %v", window.LastAt)
	}
}

func TestReceiverBreakJudgmentUnaffectedByLateFrame(t *testing.T) {
	base := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	maxGap := 500 * time.Millisecond
	state := NewState()
	if err := state.Start(1, 1, 0, base); err != nil {
		t.Fatalf("start: %v", err)
	}
	receiver := NewReceiver(state, maxGap)
	if err := receiver.Receive(frameAt("first", 1, base.Add(200*time.Millisecond))); err != nil {
		t.Fatalf("receive first frame: %v", err)
	}
	// Late frame inside the dwell: rejected without breaking the window.
	if err := receiver.Receive(frameAt("late", 1, base.Add(50*time.Millisecond))); err == nil {
		t.Fatalf("expected late frame to be rejected")
	}
	if !state.Snapshot().Valid {
		t.Fatalf("late frame must not break the window")
	}
	// The next in-order frame is judged against the confirmed last time,
	// so a gap measured from the true progress must not break early.
	if err := receiver.Receive(frameAt("next", 2, base.Add(600*time.Millisecond))); err != nil {
		t.Fatalf("next in-order frame broke early: %v", err)
	}
	window := state.Snapshot()
	if window.Samples != 2 || !window.LastAt.Equal(base.Add(600*time.Millisecond)) {
		t.Fatalf("confirmed progress lost: samples = %d, last_at = %v", window.Samples, window.LastAt)
	}
	// A genuine dwell overrun still breaks.
	if err := receiver.Receive(frameAt("stalled", 3, base.Add(1200*time.Millisecond))); err == nil {
		t.Fatalf("expected gap overrun to break the window")
	}
	if state.Snapshot().Valid {
		t.Fatalf("window must break after dwell overrun")
	}
}
