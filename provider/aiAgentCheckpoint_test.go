package provider

import (
	"testing"

	"kandaoni.com/anqicms/model"
)

func TestResumeRoundFromCheckpoint(t *testing.T) {
	if got := resumeRoundFromCheckpoint(nil); got != 0 {
		t.Fatalf("nil checkpoint → 0, got %d", got)
	}
	if got := resumeRoundFromCheckpoint(&model.AiAgentCheckpoint{Round: 5}); got != 5 {
		t.Fatalf("round 5 → 5, got %d", got)
	}
	// fail-safe：负数/异常值回落到 0，避免循环越界
	if got := resumeRoundFromCheckpoint(&model.AiAgentCheckpoint{Round: -3}); got != 0 {
		t.Fatalf("negative round → 0, got %d", got)
	}
}

func TestAiAgentCheckpointModel(t *testing.T) {
	cp := model.AiAgentCheckpoint{}
	if cp.TableName() != "ai_agent_checkpoint" {
		t.Fatalf("TableName mismatch: %s", cp.TableName())
	}
	if model.AgentCheckpointRunning != 1 || model.AgentCheckpointDone != 2 || model.AgentCheckpointFailed != 3 {
		t.Fatal("checkpoint status constants changed unexpectedly")
	}
}
