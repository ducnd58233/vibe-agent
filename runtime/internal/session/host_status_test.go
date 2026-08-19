package session

import "testing"

func TestIsCommandInjectionUserText(t *testing.T) {
	envelope := "<command-message>goal</command-message>\n<command-name>/goal</command-name>"
	if !IsCommandInjectionUserText(envelope) {
		t.Fatal("command envelope must be classified as injection")
	}
	file := "Drive one objective end to end.\n\n<context>\n\nFollow the skill.\n</context>\n\n## Inputs"
	if !IsCommandInjectionUserText(file) {
		t.Fatal("expanded command file must be classified as injection")
	}
	if IsCommandInjectionUserText("please fix the composer") {
		t.Fatal("plain user text must not be classified as injection")
	}
}

func TestEphemeralHostStatus(t *testing.T) {
	if !EphemeralHostStatus(HostStatusTimeout) {
		t.Fatal("timeout copy must be ephemeral")
	}
	if EphemeralHostStatus("real assistant reply") {
		t.Fatal("substantive reply must not be ephemeral")
	}
}
