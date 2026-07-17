package appserver

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/blueberrycongee/wuu/internal/config"
	"github.com/blueberrycongee/wuu/internal/session"
)

// An explicit process-scoped permission override (exec --permission-mode) must
// beat a thread's pinned mode: a user asking for read_only can never be
// silently escalated to a resumed session's broader pinned authority.
func TestResolveThreadTurnPermissionsExplicitOverrideBeatsThreadPin(t *testing.T) {
	rt := newTestRuntime(t, &fakeClient{})
	rt.Permissions = config.ResolvedPermissions{Mode: config.PermissionModeReadOnly}
	srv := New(rt, &lockedBuffer{})
	th := newThreadState("pinned-thread", nil, rt.ProviderName, rt.Model, rt.RootDir, false, time.Now().UTC())
	th.PermissionMode = config.PermissionModeUnconfined

	got, err := srv.resolveThreadTurnPermissions(th, nil)
	if err != nil {
		t.Fatalf("resolve without override: %v", err)
	}
	if got.Mode != config.PermissionModeUnconfined {
		t.Fatalf("thread pin should win without override, got %q", got.Mode)
	}

	rt.PermissionModeExplicit = true
	got, err = srv.resolveThreadTurnPermissions(th, nil)
	if err != nil {
		t.Fatalf("resolve with override: %v", err)
	}
	if got.Mode != config.PermissionModeReadOnly {
		t.Fatalf("explicit override should beat thread pin, got %q", got.Mode)
	}

	matching := config.PermissionModeReadOnly
	if _, err := srv.resolveThreadTurnPermissions(th, &matching); err != nil {
		t.Fatalf("requested mode equal to override should pass: %v", err)
	}
	conflicting := config.PermissionModeUnconfined
	if _, err := srv.resolveThreadTurnPermissions(th, &conflicting); err == nil {
		t.Fatal("requested mode conflicting with override should be rejected")
	}
}

func writeSelectionTestConfig(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(`{
  "default_provider": "fake-provider",
  "providers": {
    "fake-provider": {
      "type": "openai-compatible",
      "base_url": "https://example.test/v1",
      "api_key": "test-key",
      "model": "fake-model"
    }
  }
}`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

// A resident idle runtime must never run a selection it was not built for:
// when the thread's selection changes behind the runtime's back (e.g. another
// app-server process repinned the session and admission refreshed th from
// disk), reuse must detach the stale runtime and rebuild.
func TestEnsureThreadRuntimeRebuildsWhenThreadSelectionChanged(t *testing.T) {
	rt := newTestRuntime(t, &fakeClient{})
	writeSelectionTestConfig(t, rt.ConfigPath)
	out := &lockedBuffer{}
	srv := New(rt, out)

	if err := srv.handleLine(context.Background(), []byte(`{"id":"1","method":"thread/start"}`)); err != nil {
		t.Fatalf("thread/start: %v", err)
	}
	threadID := remarshal[ThreadStartResult](t, responseByID(t, parseOutput(t, out.String()), "1")["result"]).Thread.ID
	th := srv.thread(threadID)

	first, err := srv.ensureThreadRuntime(th)
	if err != nil {
		t.Fatalf("ensureThreadRuntime: %v", err)
	}
	reused, err := srv.ensureThreadRuntime(th)
	if err != nil {
		t.Fatalf("ensureThreadRuntime reuse: %v", err)
	}
	if reused != first {
		t.Fatal("unchanged selection should reuse the resident runtime")
	}

	th.mu.Lock()
	th.Model = "repinned-model"
	th.mu.Unlock()
	rebuilt, err := srv.ensureThreadRuntime(th)
	if err != nil {
		t.Fatalf("ensureThreadRuntime after model repin: %v", err)
	}
	if rebuilt == first {
		t.Fatal("model repin behind the runtime's back should force a rebuild")
	}
	if rebuilt.StreamRunner.Model != "repinned-model" {
		t.Fatalf("rebuilt runtime model = %q, want repinned-model", rebuilt.StreamRunner.Model)
	}
}

// A session pinned to a provider that has since been removed from config must
// not fail forever: the first turn self-heals the pin to the workspace
// defaults, persists the healed selection, and broadcasts it so the composer
// stops showing the dead provider.
func TestEnsureThreadRuntimeHealsRemovedProviderPin(t *testing.T) {
	rt := newTestRuntime(t, &fakeClient{})
	writeSelectionTestConfig(t, rt.ConfigPath)
	out := &lockedBuffer{}
	srv := New(rt, out)

	if _, err := session.CreateWithMetadata(rt.SessionDir, "healed-thread", rt.RootDir); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if _, err := session.SetRuntimeSelection(rt.SessionDir, "healed-thread", session.RuntimeSelection{
		Provider: "removed-provider",
		Model:    "removed-model",
	}); err != nil {
		t.Fatalf("pin removed provider: %v", err)
	}
	if err := srv.handleLine(context.Background(), []byte(`{"id":"1","method":"thread/resume","params":{"session_id":"healed-thread"}}`)); err != nil {
		t.Fatalf("thread/resume: %v", err)
	}
	th := srv.thread("healed-thread")
	if th == nil {
		t.Fatal("resumed thread is not resident")
	}

	built, err := srv.ensureThreadRuntime(th)
	if err != nil {
		t.Fatalf("ensureThreadRuntime should heal the dead pin: %v", err)
	}
	if built.StreamRunner.Model != "fake-model" {
		t.Fatalf("healed runtime model = %q, want workspace default fake-model", built.StreamRunner.Model)
	}
	th.mu.Lock()
	provider, model := th.ModelProvider, th.Model
	th.mu.Unlock()
	if provider != "fake-provider" || model != "fake-model" {
		t.Fatalf("thread selection not healed: provider=%q model=%q", provider, model)
	}
	sess, ok, err := session.Find(rt.SessionDir, "healed-thread")
	if err != nil || !ok {
		t.Fatalf("find session: ok=%v err=%v", ok, err)
	}
	if sess.Provider != "fake-provider" || sess.Model != "fake-model" {
		t.Fatalf("session row not healed: provider=%q model=%q", sess.Provider, sess.Model)
	}
	var healed *Thread
	for _, msg := range parseOutput(t, out.String()) {
		if msg["method"] != "thread/updated" {
			continue
		}
		notif := remarshal[ThreadUpdatedNotification](t, msg["params"])
		if notif.Thread.ID == "healed-thread" {
			healed = &notif.Thread
		}
	}
	if healed == nil || healed.ModelProvider != "fake-provider" || healed.Model != "fake-model" {
		t.Fatalf("no thread/updated broadcasting the healed selection, got %+v", healed)
	}
}

// Permission mode is re-resolved and applied to the toolkit on every turn, so
// pin drift on the permission dimension alone must not churn the resident
// runtime (a rebuild would break the thread's prompt-cache prefix for
// nothing).
func TestEnsureThreadRuntimePermissionPinDriftDoesNotRebuild(t *testing.T) {
	rt := newTestRuntime(t, &fakeClient{})
	rt.Permissions = config.ResolvedPermissions{Mode: config.PermissionModeReadOnly}
	out := &lockedBuffer{}
	srv := New(rt, out)

	if err := srv.handleLine(context.Background(), []byte(`{"id":"1","method":"thread/start"}`)); err != nil {
		t.Fatalf("thread/start: %v", err)
	}
	threadID := remarshal[ThreadStartResult](t, responseByID(t, parseOutput(t, out.String()), "1")["result"]).Thread.ID
	th := srv.thread(threadID)

	first, err := srv.ensureThreadRuntime(th)
	if err != nil {
		t.Fatalf("ensureThreadRuntime: %v", err)
	}
	th.mu.Lock()
	th.PermissionMode = config.PermissionModeUnconfined
	th.mu.Unlock()
	reused, err := srv.ensureThreadRuntime(th)
	if err != nil {
		t.Fatalf("ensureThreadRuntime after pin drift: %v", err)
	}
	if reused != first {
		t.Fatal("permission pin drift alone should not rebuild the resident runtime")
	}
}
