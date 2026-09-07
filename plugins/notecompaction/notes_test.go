package notecompaction

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	pluginapi "github.com/blueberrycongee/wuu/packages/plugin-go"
)

// The handler has no process-local memory. Recreate it on every call while
// retaining the host's storage, as happens after extension reloads.
type notesHost struct {
	pluginapi.Host
	values   map[string]string
	conflict bool
}

func (h *notesHost) CallHost(_ context.Context, method string, params, result any) error {
	if method != pluginapi.HostServiceCallMethod {
		return errors.New("unexpected host method")
	}
	data, _ := json.Marshal(params)
	var request struct {
		Service string
		Params  pluginapi.StorageCompareExchangeParams
	}
	if err := json.Unmarshal(data, &request); err != nil {
		return err
	}
	value, found := h.values[request.Params.Key]
	var current *string
	if found {
		current = &value
	}
	switch request.Service {
	case pluginapi.HostServiceStorageGet:
		*result.(*pluginapi.StorageGetResult) = pluginapi.StorageGetResult{Value: current}
	case pluginapi.KernelStorageCompareExchangeService:
		swapped := !h.conflict && reflect.DeepEqual(current, request.Params.Expected)
		if swapped {
			h.values[request.Params.Key] = *request.Params.Value
		}
		*result.(*pluginapi.StorageCompareExchangeResult) = pluginapi.StorageCompareExchangeResult{Swapped: swapped}
	default:
		return errors.New("unexpected service")
	}
	return nil
}

func callNotes(t *testing.T, host *notesHost, session string, input map[string]any) (map[string]any, error) {
	t.Helper()
	result, err := Handler().ExecuteTool(context.Background(), host, pluginapi.ToolCall{ToolID: toolNotes, SessionID: session, Arguments: rawMessage(t, input)})
	if err != nil {
		return nil, err
	}
	encoded, _ := json.Marshal(result)
	var envelope struct{ Content []struct{ Text string } }
	if err := json.Unmarshal(encoded, &envelope); err != nil {
		t.Fatal(err)
	}
	var output map[string]any
	if len(envelope.Content) != 1 {
		t.Fatalf("unexpected result: %s", encoded)
	}
	if err := json.Unmarshal([]byte(envelope.Content[0].Text), &output); err != nil {
		t.Fatal(err)
	}
	return output, nil
}

func TestNotesRecoverAcrossHandlersAndIsolateSessions(t *testing.T) {
	host := &notesHost{values: map[string]string{}}
	written, err := callNotes(t, host, "one", map[string]any{"action": "write", "path": "work/current.md", "content": "目标：修复恢复\nSeq 18", "revision": ""})
	if err != nil {
		t.Fatal(err)
	}
	read, err := callNotes(t, host, "one", map[string]any{"action": "read", "path": "work/current.md", "offset": 3, "limit": 4})
	if err != nil || read["content"] != "修复恢复" || read["next_offset"] != float64(7) {
		t.Fatalf("read=%v err=%v", read, err)
	}
	other, err := callNotes(t, host, "two", map[string]any{"action": "list"})
	if err != nil || other["total"] != float64(0) {
		t.Fatalf("session leaked: %v %v", other, err)
	}
	if _, err := callNotes(t, host, "one", map[string]any{"action": "append", "path": "work/current.md", "content": " stale", "revision": ""}); err == nil {
		t.Fatal("stale update accepted")
	}
	host.conflict = true
	if _, err := callNotes(t, host, "one", map[string]any{"action": "append", "path": "work/current.md", "content": " racing", "revision": written["revision"]}); err == nil {
		t.Fatal("concurrent update accepted")
	}
	host.conflict = false
	updated, err := callNotes(t, host, "one", map[string]any{"action": "append", "path": "work/current.md", "content": "\nverified", "revision": written["revision"]})
	if err != nil || updated["revision"] == written["revision"] {
		t.Fatalf("append=%v %v", updated, err)
	}
	if _, err := callNotes(t, host, "one", map[string]any{"action": "read", "path": "work/current.md", "offset": 7, "revision": written["revision"]}); err == nil {
		t.Fatal("stale pagination accepted")
	}
}

func TestNotesSearchAndReadBoundedPages(t *testing.T) {
	host := &notesHost{values: map[string]string{}}
	_, err := callNotes(t, host, "one", map[string]any{"action": "write", "path": "work.md", "content": strings.Repeat("中<&", 30000), "revision": ""})
	if err != nil {
		t.Fatal(err)
	}
	page, err := callNotes(t, host, "one", map[string]any{"action": "search", "query": "<&", "offset": 29998, "limit": 2})
	if err != nil || page["total"] != float64(30000) {
		t.Fatalf("search=%v %v", page, err)
	}
	items := page["items"].([]any)
	if len(items) != 2 || items[0].(map[string]any)["offset"] != float64(29998*3+1) {
		t.Fatalf("items=%v", items)
	}
	page, err = callNotes(t, host, "one", map[string]any{"action": "search", "query": "<&", "limit": 100})
	encoded, _ := json.Marshal(page)
	if err != nil || len(encoded) > 18000 || page["next_offset"] == nil {
		t.Fatalf("unbounded search: %d %v", len(encoded), err)
	}
	page, err = callNotes(t, host, "one", map[string]any{"action": "read", "path": "work.md", "limit": 16000})
	encoded, _ = json.Marshal(page)
	if err != nil || len(encoded) > 18000 || page["next_offset"] == nil {
		t.Fatalf("unbounded read: %d %v", len(encoded), err)
	}
}

func TestNotesRejectInvalidAndOversizedWrites(t *testing.T) {
	for _, input := range []map[string]any{
		{"action": "write", "path": "../x", "content": "x", "revision": ""},
		{"action": "write", "path": "x", "content": "x"},
		{"action": "write", "path": "x", "content": strings.Repeat("x", maxNotesBytes), "revision": ""},
		{"action": "read", "offset": -1},
	} {
		host := &notesHost{values: map[string]string{}}
		if _, err := callNotes(t, host, "one", input); err == nil || len(host.values) != 0 {
			t.Fatal("invalid request mutated storage")
		}
	}
}
