package appserver

import (
	"testing"

	"github.com/blueberrycongee/wuu/internal/providers"
)

func TestUserMessageFromPromptPreservesValidContentParts(t *testing.T) {
	prompt := "pasted body\nregular text"
	parts := []providers.MessageContentPart{
		{Type: "pasted_text", Text: "pasted body\n"},
		{Type: "text", Text: "regular text"},
	}

	msg, err := userMessageFromPrompt(prompt, nil, nil, parts)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Content != prompt {
		t.Fatalf("Content = %q, want %q", msg.Content, prompt)
	}
	if len(msg.ContentParts) != 2 || msg.ContentParts[0].Type != "pasted_text" {
		t.Fatalf("ContentParts = %#v, want structured pasted and text parts", msg.ContentParts)
	}
}

func TestUserMessageFromPromptRejectsMismatchedContentParts(t *testing.T) {
	msg, err := userMessageFromPrompt(
		"canonical prompt",
		nil,
		nil,
		[]providers.MessageContentPart{{Type: "pasted_text", Text: "different text"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(msg.ContentParts) != 0 {
		t.Fatalf("ContentParts = %#v, want no presentation metadata for mismatched content", msg.ContentParts)
	}
}
