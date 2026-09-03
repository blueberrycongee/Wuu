package toolledger

import (
	"context"
	"testing"

	"github.com/blueberrycongee/wuu/internal/providers"
	"github.com/blueberrycongee/wuu/internal/session"
	"github.com/blueberrycongee/wuu/internal/toolresult"
)

func TestProjectedInvocationDropsRecoveryPayload(t *testing.T) {
	dir := t.TempDir()
	ledger, err := New(dir, "thread-projected-storage")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	batchID, err := ledger.BeginBatch(ctx, "operation-1", 1)
	if err != nil {
		t.Fatal(err)
	}
	invocation, err := ledger.Prepare(ctx, batchID, providers.ToolCall{ID: "call-1", Name: "read_file", Arguments: `{}`}, ReplayAtMostOnce)
	if err != nil {
		t.Fatal(err)
	}
	if err := ledger.FinalizeBatch(ctx, batchID); err != nil {
		t.Fatal(err)
	}
	if err := ledger.Start(ctx, invocation.ID); err != nil {
		t.Fatal(err)
	}
	result := toolresult.FromText("durable result")
	if err := ledger.Settle(ctx, invocation.ID, result); err != nil {
		t.Fatal(err)
	}
	if err := ledger.MarkProjected(ctx, []string{invocation.ID}); err != nil {
		t.Fatal(err)
	}
	db, err := session.OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	var stored string
	if err := db.QueryRow(`SELECT result_json FROM tool_invocations WHERE id = ?`, invocation.ID).Scan(&stored); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if stored != "" {
		t.Fatalf("projected recovery payload was retained: %q", stored)
	}
	if err := ledger.Settle(ctx, invocation.ID, result); err != nil {
		t.Fatalf("idempotent settlement after payload cleanup: %v", err)
	}
}
