package appserver

import (
	"strings"
	"testing"
	"time"

	"github.com/blueberrycongee/wuu/internal/agent"
	"github.com/blueberrycongee/wuu/internal/automation"
)

func TestAutomationRequestContextIsHiddenAndRequestOnly(t *testing.T) {
	segments := automationRequestContext(
		automation.Task{ID: "task-1", Cron: "*/5 * * * *", Timezone: "UTC", Mode: string(automation.ModeThreadHeartbeat)},
		automation.Run{ID: "run-1", TriggeredAt: time.Date(2026, 7, 16, 7, 0, 0, 0, time.UTC)},
	)
	if len(segments) != 1 {
		t.Fatalf("segments = %#v", segments)
	}
	segment := segments[0]
	if segment.Lifecycle != agent.ContextSegmentRequestOnly || segment.Durable || segment.VisibleInUI {
		t.Fatalf("automation context must be hidden and request-only: %#v", segment)
	}
	if len(segment.Messages) != 1 || !strings.Contains(segment.Messages[0].Content, "task_id: task-1") || !strings.Contains(segment.Messages[0].Content, "run_id: run-1") {
		t.Fatalf("automation context message = %#v", segment.Messages)
	}
}
