package session

import "testing"

func TestNormalizeTypeLegacyTranscriptMessage(t *testing.T) {
	if got := NormalizeType(legacyTypeTranscriptMessage, "thinking"); got != TypeThinking {
		t.Errorf("thinking role -> %q, want %q", got, TypeThinking)
	}
	if got := NormalizeType(legacyTypeTranscriptMessage, "assistant"); got != TypeMessage {
		t.Errorf("assistant role -> %q, want %q", got, TypeMessage)
	}
	if got := NormalizeType(TypeMessage, "thinking"); got != TypeMessage {
		t.Errorf("already-split message must stay message, got %q", got)
	}
}

func TestTypeForChatRole(t *testing.T) {
	if TypeForChatRole("thinking") != TypeThinking {
		t.Fatal("expected thinking")
	}
	if TypeForChatRole("assistant") != TypeMessage {
		t.Fatal("expected message")
	}
}
