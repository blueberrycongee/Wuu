package appserver

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/blueberrycongee/wuu/internal/agent"
	"github.com/blueberrycongee/wuu/internal/execution"
	"github.com/blueberrycongee/wuu/internal/hooks"
	"github.com/blueberrycongee/wuu/internal/providers"
	"github.com/blueberrycongee/wuu/internal/session"
	"github.com/blueberrycongee/wuu/internal/structuredoutput"
)

func TestRunStartSettlesManifestBeforeRunRead(t *testing.T) {
	client := &fakeClient{response: providersResponse("run-protocol-ok")}
	rt := newTestRuntime(t, client)
	runtimeJournal, err := session.NewInferenceJournalRuntime(rt.SessionDir, "run-test")
	if err != nil {
		t.Fatalf("NewInferenceJournalRuntime: %v", err)
	}
	rt.InferenceJournalRuntime = runtimeJournal
	defer runtimeJournal.Close()
	rt.StreamRunner = &agent.StreamRunner{
		Client:       providers.AdaptStreamClient(client),
		ProviderName: rt.ProviderName,
		Model:        rt.Model,
		SystemPrompt: "system",
	}
	rt.HookDispatcher = hooks.NewDispatcher(nil)

	out := &lockedBuffer{}
	srv := New(rt, out)
	defer srv.Close()
	thread := newThreadState("ephemeral-run-thread", []providers.ChatMessage{{Role: "system", Content: "system"}}, rt.ProviderName, rt.Model, rt.RootDir, false, time.Now().UTC())
	thread.Ephemeral = true
	srv.threads[thread.ID] = thread

	request := Request{ID: json.RawMessage(`"start"`), Method: MethodRunStart, Params: mustJSON(RunStartParams{
		ThreadID: thread.ID,
		Prompt:   "reply with exactly: run-protocol-ok",
		Request:  execution.Request{Mode: execution.ModeStart, Requested: execution.Selection{Provider: "fake-provider", Model: "fake-model"}},
	})}
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.handleLine(context.Background(), raw); err != nil {
		t.Fatalf("run/start: %v", err)
	}
	startResult := remarshal[RunStartResult](t, responseByID(t, parseOutput(t, out.String()), "start")["result"])
	run := startResult.Run

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		stored, getErr := srv.runStore.Get(context.Background(), run.ID)
		if getErr == nil {
			run = stored
			if run.Status.Terminal() {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if run.ID == "" {
		t.Fatalf("run/start did not return a Run: %s", out.String())
	}
	if run.Status != execution.StatusCompleted {
		t.Fatalf("Run status = %q, output: %s", run.Status, out.String())
	}
	if len(run.Turns) != 1 || run.Turns[0].TurnID == "" {
		t.Fatalf("Run turn refs = %+v", run.Turns)
	}
	if run.Request.HasPrompt != true || run.Request.ImageCount != 0 {
		t.Fatalf("Run request facts = %+v", run.Request)
	}
	if run.Runtime.Resolved.Provider != "fake-provider" || run.Runtime.Resolved.Model != "fake-model" {
		t.Fatalf("Run runtime facts = %+v", run.Runtime)
	}
	if run.Result == nil || run.Result.FinalTurnID != run.Turns[0].TurnID {
		t.Fatalf("Run result = %+v", run.Result)
	}

	readRaw := mustJSON(Request{ID: json.RawMessage(`"read"`), Method: MethodRunRead, Params: mustJSON(RunReadParams{RunID: run.ID})})
	if err := srv.handleLine(context.Background(), readRaw); err != nil {
		t.Fatalf("run/read: %v", err)
	}
	readResult := remarshal[RunReadResult](t, responseByID(t, parseOutput(t, out.String()), "read")["result"])
	if readResult.Run.Status != execution.StatusCompleted || readResult.Thread == nil {
		t.Fatalf("run/read result = %+v", readResult)
	}
	if readResult.Attached {
		t.Fatalf("terminal Run should not remain attached")
	}

	listRaw := mustJSON(Request{ID: json.RawMessage(`"list"`), Method: MethodRunList, Params: mustJSON(RunListParams{ThreadID: thread.ID})})
	if err := srv.handleLine(context.Background(), listRaw); err != nil {
		t.Fatalf("run/list: %v", err)
	}
	listResult := remarshal[RunListResult](t, responseByID(t, parseOutput(t, out.String()), "list")["result"])
	if len(listResult.Runs) != 1 || listResult.Runs[0].ID != run.ID {
		t.Fatalf("run/list result = %+v", listResult)
	}
}

func TestExecutionRunSchemaOutcomeRetriesWithinOneRun(t *testing.T) {
	srv := &Server{runs: make(map[string]*runTracker), activeRunByThread: make(map[string]string)}
	validator, err := structuredoutput.New(json.RawMessage(`{"type":"object","required":["ok"]}`))
	if err != nil {
		t.Fatalf("New validator: %v", err)
	}
	srv.registerExecutionRun(execution.Run{ID: "run-schema", ThreadID: "thread-schema"}, validator)

	for attempt := 0; attempt < structuredoutput.MaxRetries; attempt++ {
		prompt, retry, validationErr := srv.executionRunSchemaOutcome("run-schema", `{"wrong":true}`)
		if !retry || validationErr != nil || prompt == "" {
			t.Fatalf("attempt %d outcome = prompt %q, retry %v, error %v", attempt, prompt, retry, validationErr)
		}
	}
	_, retry, validationErr := srv.executionRunSchemaOutcome("run-schema", `{"wrong":true}`)
	if retry || validationErr == nil {
		t.Fatalf("exhausted outcome = retry %v, error %v", retry, validationErr)
	}
	if _, retry, validationErr := srv.executionRunSchemaOutcome("run-schema", `{"ok":true}`); retry || validationErr != nil {
		t.Fatalf("valid outcome = retry %v, error %v", retry, validationErr)
	}
}
