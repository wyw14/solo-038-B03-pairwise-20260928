package register

import (
	"testing"
	"time"

	"github.com/wyw14/solo-038/internal/sensor"
)

func frameAt(id string, meter float64, at time.Time) sensor.CameraFrame {
	return sensor.CameraFrame{ID: id, Camera: "cam", JobGeneration: 1, PlateGeneration: 1, WebMeter: meter, CapturedAt: at, ReceivedAt: at}
}

func TestOutOfOrderFrameKeepsProgress(t *testing.T) {
	base := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)
	state := NewState()
	if err := state.Start(1, 1, 0, base); err != nil {
		t.Fatal(err)
	}
	receiver := NewReceiver(state, 500*time.Millisecond)

	if err := receiver.Receive(frameAt("a", 1, base.Add(100*time.Millisecond))); err != nil {
		t.Fatal(err)
	}
	if err := receiver.Receive(frameAt("b", 2, base.Add(200*time.Millisecond))); err != nil {
		t.Fatal(err)
	}

	late := frameAt("late", 3, base.Add(50*time.Millisecond))
	if err := receiver.Receive(late); err != nil {
		t.Fatalf("late frame should be ignored, not rejected: %v", err)
	}
	window := state.Snapshot()
	if window.Samples != 2 {
		t.Fatalf("late frame extended stable window: samples=%d", window.Samples)
	}
	if !window.LastAt.Equal(base.Add(200 * time.Millisecond)) {
		t.Fatalf("window last time regressed: %v", window.LastAt)
	}
	if window.LastMeter != 2 {
		t.Fatalf("window last meter changed: %v", window.LastMeter)
	}

	if err := receiver.Receive(frameAt("c", 4, base.Add(600*time.Millisecond))); err != nil {
		t.Fatalf("gap should be measured from confirmed last time: %v", err)
	}
	if window := state.Snapshot(); window.Samples != 3 {
		t.Fatalf("in-order frame not counted: samples=%d", window.Samples)
	}

	if err := receiver.Receive(frameAt("d", 5, base.Add(1200*time.Millisecond))); err == nil {
		t.Fatal("expected break after dwell exceeded")
	}
	if state.Snapshot().Valid {
		t.Fatal("window should be broken after gap")
	}
}
