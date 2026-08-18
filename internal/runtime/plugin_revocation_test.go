package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/blueberrycongee/wuu/internal/agent"
	"github.com/blueberrycongee/wuu/internal/pluginhost"
)

type failingCloseClient struct {
	*generationClient
	closeErr error
}

func (c *failingCloseClient) Close(context.Context) error {
	c.closed = true
	return c.closeErr
}

func TestGenerationCloseRecordsStructuredRevocationReport(t *testing.T) {
	okClient := &generationClient{id: "ok"}
	brokenClient := &failingCloseClient{generationClient: &generationClient{id: "broken"}, closeErr: errors.New("process refused to exit")}
	generation := &PluginGeneration{
		id:            "gen-test-close",
		host:          pluginhost.New(okClient, brokenClient),
		systemPrompts: agent.NewSystemPromptAssembler(),
		compactions:   agent.NewCompactionRegistry(),
	}
	ownedRoot := t.TempDir()
	ownedRoots := []string{filepath.Join(ownedRoot, "snapshot-a"), filepath.Join(ownedRoot, "snapshot-b")}
	generation.ownedRoots = ownedRoots
	for _, root := range ownedRoots {
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	err := generation.close()
	if err == nil {
		t.Fatal("close unexpectedly succeeded")
	}
	report := generation.revocationReport()
	if report == nil || report.GenerationID != "gen-test-close" || report.RetiredAt.IsZero() {
		t.Fatalf("report identity missing: %+v", report)
	}
	if !report.Failed() {
		t.Fatal("report did not flag the failed plugin process shutdown")
	}
	var processRecords, snapshotRecords []ResourceRevocation
	for _, record := range report.Resources {
		switch record.Resource {
		case "plugin-process":
			processRecords = append(processRecords, record)
		case "package-snapshot":
			snapshotRecords = append(snapshotRecords, record)
		}
		if record.GenerationID != "gen-test-close" {
			t.Fatalf("record lost generation attribution: %+v", record)
		}
	}
	if len(processRecords) != 2 {
		t.Fatalf("expected one record per plugin process, got %+v", report.Resources)
	}
	// Clients retire in reverse start order: broken closed before ok.
	if processRecords[0].PluginID != "broken" || processRecords[0].Outcome != RevocationOutcomeFailed || processRecords[0].Detail == "" {
		t.Fatalf("failed client record = %+v", processRecords[0])
	}
	if processRecords[1].PluginID != "ok" || processRecords[1].Outcome != RevocationOutcomeRevoked {
		t.Fatalf("healthy client record = %+v", processRecords[1])
	}
	if len(snapshotRecords) != 2 || snapshotRecords[0].Detail == "" {
		t.Fatalf("snapshot records lost their roots: %+v", snapshotRecords)
	}
	if _, statErr := os.Stat(ownedRoots[0]); !os.IsNotExist(statErr) {
		t.Fatal("owned package snapshot survived retirement")
	}
}

func TestActivatePluginGenerationPersistsRevocationReports(t *testing.T) {
	wuuHome := t.TempDir()
	oldClient := &generationClient{id: "old"}
	old := testPluginGeneration("old", oldClient)
	session := testGenerationSession(old)
	session.WuuHome = wuuHome

	broken := &preparedGenerationClient{
		generationClient: &generationClient{id: "broken"},
		activateErr:      errors.New("activate failed"),
	}
	failedCandidate := testPluginGeneration("broken", broken)
	if err := session.ActivatePluginGeneration(failedCandidate, nil); err == nil {
		t.Fatal("activation unexpectedly succeeded")
	}

	candidate := testPluginGeneration("candidate", &generationClient{id: "candidate"})
	if err := session.ActivatePluginGeneration(candidate, nil); err != nil {
		t.Fatal(err)
	}

	reports, err := session.PluginGenerationRevocations(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 2 {
		t.Fatalf("expected failed candidate and retired old generation reports, got %+v", reports)
	}
	// Newest first: the retired old generation, then the rejected candidate.
	if reports[0].GenerationID != old.id || reports[0].Failed() {
		t.Fatalf("retired old generation report = %+v", reports[0])
	}
	if reports[1].GenerationID != failedCandidate.id {
		t.Fatalf("rejected candidate report = %+v", reports[1])
	}
	limited, err := session.PluginGenerationRevocations(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(limited) != 1 || limited[0].GenerationID != old.id {
		t.Fatalf("limited reports = %+v", limited)
	}
}

func TestGenerationRevocationPersistenceSkipsCorruptLines(t *testing.T) {
	wuuHome := t.TempDir()
	report := &GenerationRevocationReport{GenerationID: "gen-kept", RetiredAt: time.Now().UTC()}
	if err := appendGenerationRevocation(wuuHome, report); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(wuuHome, pluginGenerationRevocationFile)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("{not json\n"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	reports, err := readGenerationRevocations(wuuHome, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || reports[0].GenerationID != "gen-kept" {
		t.Fatalf("reports = %+v", reports)
	}
}
