package plugin

import (
	"testing"
)

func TestSeamDispatchValidate(t *testing.T) {
	tests := []struct {
		name     string
		dispatch SeamDispatch
		wantErr  bool
	}{
		{
			name:     "empty kind",
			dispatch: SeamDispatch{},
			wantErr:  true,
		},
		{
			name: "valid observe",
			dispatch: SeamDispatch{
				Kind: SeamObserve, Concurrent: true, ErrorPolicy: ErrorPolicyIgnore,
			},
			wantErr: false,
		},
		{
			name: "observe without concurrent is ok",
			dispatch: SeamDispatch{
				Kind: SeamObserve, ErrorPolicy: ErrorPolicyIgnore,
			},
			wantErr: false,
		},
		{
			name: "concurrent on non-observe",
			dispatch: SeamDispatch{
				Kind: SeamTransform, Concurrent: true,
			},
			wantErr: true,
		},
		{
			name: "short-circuit on guard",
			dispatch: SeamDispatch{
				Kind: SeamGuard, ShortCircuit: true, ErrorPolicy: ErrorPolicyPropagate,
			},
			wantErr: false,
		},
		{
			name: "short-circuit on decision",
			dispatch: SeamDispatch{
				Kind: SeamDecision, ShortCircuit: true, ErrorPolicy: ErrorPolicyPropagate,
			},
			wantErr: false,
		},
		{
			name: "short-circuit on non-guard/decision",
			dispatch: SeamDispatch{
				Kind: SeamTransform, ShortCircuit: true,
			},
			wantErr: true,
		},
		{
			name: "error ignore on non-observe",
			dispatch: SeamDispatch{
				Kind: SeamTransform, ErrorPolicy: ErrorPolicyIgnore,
			},
			wantErr: true,
		},
		{
			name: "valid transform",
			dispatch: SeamDispatch{
				Kind: SeamTransform, Ordered: true, ErrorPolicy: ErrorPolicyPropagate,
			},
			wantErr: false,
		},
		{
			name: "valid around",
			dispatch: SeamDispatch{
				Kind: SeamAround, ErrorPolicy: ErrorPolicyPropagate,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.dispatch.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestSeamCatalogRegisterAndGet(t *testing.T) {
	cat := NewSeamCatalog()

	seam := Seam{
		Name: "agent.tool.execute.around",
		Dispatch: SeamDispatch{
			Kind: SeamAround, ErrorPolicy: ErrorPolicyPropagate,
		},
	}

	err := cat.Register(seam)
	if err != nil {
		t.Fatalf("unexpected register error: %v", err)
	}

	got, ok := cat.Get("agent.tool.execute.around")
	if !ok {
		t.Fatal("expected seam to exist")
	}
	if got.Name != seam.Name {
		t.Errorf("expected name %s, got %s", seam.Name, got.Name)
	}
	if got.Dispatch.Kind != seam.Dispatch.Kind {
		t.Errorf("expected kind %s, got %s", seam.Dispatch.Kind, got.Dispatch.Kind)
	}
}

func TestSeamCatalogDuplicateRejected(t *testing.T) {
	cat := NewSeamCatalog()
	seam := Seam{
		Name: "agent.tool.register",
		Dispatch: SeamDispatch{
			Kind: SeamTransform, ErrorPolicy: ErrorPolicyPropagate,
		},
	}

	if err := cat.Register(seam); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if err := cat.Register(seam); err == nil {
		t.Fatal("expected error for duplicate seam")
	}
}

func TestSeamCatalogGetMissing(t *testing.T) {
	cat := NewSeamCatalog()
	_, ok := cat.Get("nonexistent")
	if ok {
		t.Error("expected Get to return false for missing seam")
	}
}

func TestRegisterStandardSeams(t *testing.T) {
	cat := NewSeamCatalog()
	err := RegisterStandardSeams(cat)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify a few key seams exist.
	for _, name := range []string{
		"agent.tool.register",
		"agent.system_prompt.section",
		"agent.compaction",
		"agent.provider.register",
		"agent.subagent.provider",
		"agent.permission.policy",
		"agent.session.lifecycle",
		"desktop.view.register",
		"desktop.theme.register",
	} {
		if _, ok := cat.Get(name); !ok {
			t.Errorf("expected standard seam %s to be registered", name)
		}
	}

	// Double registration should fail.
	err = RegisterStandardSeams(cat)
	if err == nil {
		t.Error("expected error on duplicate registration")
	}
}

func TestIsSafetyKernelSeam(t *testing.T) {
	safetySeams := []string{
		"host.plugin.install",
		"host.plugin.approval",
		"host.plugin.enable",
		"host.plugin.disable",
		"host.plugin.upgrade",
		"host.plugin.delete",
		"host.safe_mode",
		"host.crash_recovery",
		"host.permission.final",
		"host.window.lifecycle",
		"host.appserver.lifecycle",
		"host.generation.isolate",
		"host.escape.settings",
		"host.escape.default_ui",
	}

	for _, name := range safetySeams {
		if !IsSafetyKernelSeam(name) {
			t.Errorf("expected %s to be a safety kernel seam", name)
		}
	}

	nonSafetySeams := []string{
		"agent.tool.register",
		"desktop.view.register",
		"agent.system_prompt.section",
	}

	for _, name := range nonSafetySeams {
		if IsSafetyKernelSeam(name) {
			t.Errorf("expected %s NOT to be a safety kernel seam", name)
		}
	}
}

func TestSeamKindString(t *testing.T) {
	tests := map[SeamKind]string{
		SeamObserve:   "observe",
		SeamTransform: "transform",
		SeamGuard:     "guard",
		SeamAround:    "around",
		SeamDecision:  "decision",
	}
	for kind, want := range tests {
		if got := kind.String(); got != want {
			t.Errorf("%s.String() = %s, want %s", kind, got, want)
		}
	}
}
