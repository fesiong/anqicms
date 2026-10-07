package provider

import (
	"testing"
	"time"
)

func TestTraceRecorderRingBuffer(t *testing.T) {
	r := NewTraceRecorder(3)
	for i := 0; i < 5; i++ {
		r.RecordPhase(TraceRound, "r", "e")
	}
	snap := r.Snapshot(0)
	if len(snap) != 3 {
		// 环形缓冲：只保留最近 3 条
		t.Fatalf("ring buffer cap=3 → 3 events, got %d", len(snap))
	}
}

func TestTraceRecorderSnapshotLimit(t *testing.T) {
	r := NewTraceRecorder(10)
	for i := 0; i < 10; i++ {
		r.RecordPhase(TraceTool, "t", "e")
	}
	if len(r.Snapshot(5)) != 5 {
		t.Fatal("Snapshot(5) should return at most 5")
	}
	// limit<=0 返回全部
	if len(r.Snapshot(0)) != 10 {
		t.Fatal("Snapshot(0) should return all")
	}
}

func TestTraceRecorderDisabled(t *testing.T) {
	r := NewTraceRecorder(5)
	r.SetEnabled(false)
	r.RecordPhase(TraceRound, "x", "y")
	if len(r.Snapshot(0)) != 0 {
		t.Fatal("disabled recorder must not store events")
	}
}

func TestTraceRecorderTypedHelpers(t *testing.T) {
	r := NewTraceRecorder(5)
	r.RecordTool("edit_file", 12*time.Millisecond, true, "denied")
	r.RecordGuardrail("write_gate", "deny", "needs approval")
	snap := r.Snapshot(0)
	if len(snap) != 2 {
		t.Fatalf("expected 2 events, got %d", len(snap))
	}
	if snap[0].Phase != TraceTool || snap[0].DurationMs != 12 || !snap[0].Err {
		t.Fatalf("tool event wrong: %+v", snap[0])
	}
	if snap[1].Phase != TraceGuardrail || snap[1].Decision != "deny" {
		t.Fatalf("guardrail event wrong: %+v", snap[1])
	}
}
