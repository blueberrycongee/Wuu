package coordinator

import (
	"strings"
	"testing"
)

// The presets are constants meant to be pasted into worker prompts
// verbatim. These tests pin the load-bearing lines so a casual edit
// can't accidentally weaken the failure-mode-specific guarantees
// the prompts encode (see presets.go for the rationale on each).

func TestVerificationPreset_HasFrameInversion(t *testing.T) {
	// The "NOT to confirm... TRY TO BREAK IT" line is what flips the
	// model out of helpful-confirmer mode. If this disappears the
	// preset stops doing its main job.
	if !strings.Contains(VerificationPreset, "NOT to confirm") {
		t.Error("VerificationPreset is missing the frame-inversion line")
	}
	if !strings.Contains(VerificationPreset, "TRY TO BREAK IT") {
		t.Error("VerificationPreset is missing the TRY TO BREAK IT directive")
	}
}

func TestVerificationPreset_RequiresEvidence(t *testing.T) {
	// "Never write PASS without evidence" is the rule that prevents
	// the verifier from rubber-stamping. Without it the model will
	// declare PASS based on plausibility instead of command output.
	if !strings.Contains(VerificationPreset, `Never write "PASS"`) {
		t.Error("VerificationPreset must require evidence for PASS verdicts")
	}
}

func TestVerificationPreset_VerdictFormat(t *testing.T) {
	// The coordinator may grep for VERDICT lines when synthesizing
	// across multiple verifiers, so the format strings must stay
	// stable. Pin all three.
	for _, want := range []string{"VERDICT: PASS", "VERDICT: FAIL", "VERDICT: PARTIAL"} {
		if !strings.Contains(VerificationPreset, want) {
			t.Errorf("VerificationPreset missing verdict literal %q", want)
		}
	}
}

func TestResearchPreset_IsReadOnly(t *testing.T) {
	// The hard "do not mutate" rule is what makes this preset safe
	// to use in a worker that has full write tools — without it
	// the model will sometimes run installs or commits "as part of
	// the investigation".
	if !strings.Contains(ResearchPreset, "Do NOT modify") {
		t.Error("ResearchPreset must forbid modifications")
	}
	if !strings.Contains(ResearchPreset, "do not mutate") &&
		!strings.Contains(ResearchPreset, "mutate state") {
		t.Error("ResearchPreset must forbid state-mutating commands")
	}
}

func TestResearchPreset_RequiresFileLineCitations(t *testing.T) {
	// The coordinator turns research output into follow-up specs;
	// without file:line refs it has nothing concrete to act on.
	if !strings.Contains(ResearchPreset, "file:line") {
		t.Error("ResearchPreset must require file:line citations")
	}
}

func TestResearchPreset_OutputShape(t *testing.T) {
	// The three-section shape (Answer / Evidence / Notes) is what
	// the coordinator relies on when reading multiple parallel
	// research results. Pin the headers.
	for _, want := range []string{"## Answer", "## Evidence", "## Notes"} {
		if !strings.Contains(ResearchPreset, want) {
			t.Errorf("ResearchPreset missing required section %q", want)
		}
	}
}

func TestSystemPromptPreamble_EmbedsBothPresets(t *testing.T) {
	// The preamble must include the preset text verbatim — that's
	// the only way the coordinator knows to copy them into worker
	// prompts. Reference-by-name wouldn't work; the model has to
	// see the actual block to paste it.
	preamble := SystemPromptPreamble()
	if !strings.Contains(preamble, VerificationPreset) {
		t.Error("SystemPromptPreamble does not embed VerificationPreset verbatim")
	}
	if !strings.Contains(preamble, ResearchPreset) {
		t.Error("SystemPromptPreamble does not embed ResearchPreset verbatim")
	}
	// And it must instruct the model to copy them verbatim.
	if !strings.Contains(preamble, "VERBATIM") {
		t.Error("SystemPromptPreamble does not tell the model to copy presets verbatim")
	}
}
