package harness

import "testing"

// hostWhere returns the first host in the contract table whose row satisfies
// want. Tests pick a host by what it can do, never by name: a rule about flat
// context or permission refusals then covers whichever hosts have that shape,
// including one added later.
func hostWhere(t *testing.T, want func(HostContract) bool) Client {
	t.Helper()
	for _, contract := range HostContracts() {
		if want(contract) {
			return contract.Client
		}
	}
	t.Fatal("no host in the contract table has the capability this test needs")
	return ""
}

func refusesWith(shape RefusalShape) func(HostContract) bool {
	return func(c HostContract) bool { return c.Dialect.Refusal == shape }
}

func contextIs(shape ContextShape) func(HostContract) bool {
	return func(c HostContract) bool { return c.Dialect.Context == shape }
}

func stopsWith(shape StopShape) func(HostContract) bool {
	return func(c HostContract) bool { return c.Dialect.StopBlock == shape }
}

func cannotInjectAtPrompt(c HostContract) bool { return !c.Dialect.PromptInjection }

func remindsAfterToolUse(c HostContract) bool { return c.Dialect.ToolUseNodeReminder }
