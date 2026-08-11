package processsandbox

import (
	"context"
	"errors"
	"os/exec"
	"reflect"
	"testing"
)

type testProvider struct {
	result ConfinedCommand
	err    error
}

func (p testProvider) Confine(context.Context, []string, Policy) (ConfinedCommand, error) {
	return p.result, p.err
}

func TestApplyWithProviderInstallsFullConfinement(t *testing.T) {
	cmd := exec.Command("/bin/echo", "hello")
	err := ApplyWithProvider(context.Background(), cmd, Policy{Mode: ModeReadOnly}, testProvider{result: ConfinedCommand{
		Argv: []string{"/usr/bin/env", "wrapped", "/bin/echo", "hello"}, Enforcement: EnforcementFull,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Path != "/usr/bin/env" || !reflect.DeepEqual(cmd.Args, []string{"/usr/bin/env", "wrapped", "/bin/echo", "hello"}) {
		t.Fatalf("command = %q %+v", cmd.Path, cmd.Args)
	}
}

func TestApplyWithProviderFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name     string
		provider Provider
	}{
		{name: "provider error", provider: testProvider{err: errors.New("offline")}},
		{name: "empty argv", provider: testProvider{result: ConfinedCommand{Enforcement: EnforcementFull}}},
		{name: "relative executable", provider: testProvider{result: ConfinedCommand{Argv: []string{"wrapper"}, Enforcement: EnforcementFull}}},
		{name: "partial", provider: testProvider{result: ConfinedCommand{Argv: []string{"/usr/bin/env"}, Enforcement: EnforcementPartial}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command("/bin/echo", "hello")
			beforePath, beforeArgs := cmd.Path, append([]string(nil), cmd.Args...)
			if err := ApplyWithProvider(context.Background(), cmd, Policy{Mode: ModeReadOnly}, tc.provider); err == nil {
				t.Fatal("expected failure")
			}
			if cmd.Path != beforePath || !reflect.DeepEqual(cmd.Args, beforeArgs) {
				t.Fatalf("failed provider mutated command: %q %+v", cmd.Path, cmd.Args)
			}
		})
	}
}
