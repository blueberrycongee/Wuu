package insight

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Run is the top-level orchestrator. It sends ProgressEvent updates through
// the channel and closes it when done.
func Run(ctx context.Context, cfg RunConfig, progress chan<- ProgressEvent) {
	defer close(progress)

	if cfg.MaxSessions <= 0 {
		cfg.MaxSessions = 50
	}
	cacheDir := CacheDir(cfg.WorkspaceRoot)

	// Phase 1: Scan sessions.
	send(progress, "scanning", "Scanning sessions...", 0.05)

	metas, err := ScanSessions(cfg.SessionDir, cfg.MaxSessions)
	if err != nil {
		sendErr(progress, fmt.Errorf("scan sessions: %w", err))
		return
	}
	if len(metas) == 0 {
		sendErr(progress, fmt.Errorf("no sessions found in %s", cfg.SessionDir))
		return
	}

	send(progress, "scanning", fmt.Sprintf("Found %d sessions", len(metas)), 0.10)

	// Cache session metadata.
	for _, m := range metas {
		_ = SaveCachedMeta(cacheDir, m)
	}

	// Phase 2: Extract facets via LLM.
	facets := make(map[string]Facet)
	uncached := 0

	// Load cached facets first.
	for _, m := range metas {
		if cached := LoadCachedFacet(cacheDir, m.ID); cached != nil {
			facets[m.ID] = *cached
		} else {
			uncached++
		}
	}

	if uncached > 0 {
		// Cap facet extractions to control cost.
		maxExtractions := 50
		if uncached > maxExtractions {
			uncached = maxExtractions
		}

		extracted := 0
		for _, m := range metas {
			if ctx.Err() != nil {
				sendErr(progress, ctx.Err())
				return
			}
			if _, ok := facets[m.ID]; ok {
				continue // already cached
			}
			if extracted >= maxExtractions {
				break
			}

			pct := 0.10 + 0.60*float64(extracted)/float64(uncached)
			send(progress, "extracting",
				fmt.Sprintf("Extracting facets (%d/%d)...", extracted+1, uncached), pct)

			transcript, err := FormatTranscript(cfg.SessionDir, m.ID)
			if err != nil {
				extracted++
				continue
			}

			facet, err := ExtractFacet(ctx, cfg.Client, cfg.Model, m.ID, transcript)
			if err != nil {
				extracted++
				continue // non-fatal
			}

			facets[m.ID] = facet
			_ = SaveCachedFacet(cacheDir, facet)
			extracted++
		}
	}

	send(progress, "extracting",
		fmt.Sprintf("Facets: %d sessions analyzed", len(facets)), 0.70)

	// Phase 3: Aggregate data.
	agg := Aggregate(metas, facets)

	// Phase 4: Generate insights via LLM.
	send(progress, "generating", "Generating insights...", 0.75)

	sections, glance, err := GenerateInsights(ctx, cfg.Client, cfg.Model, agg, facets)
	if err != nil {
		sendErr(progress, fmt.Errorf("generate insights: %w", err))
		return
	}

	send(progress, "synthesizing", "Generating HTML report...", 0.95)

	report := &Report{
		AtAGlance:   glance,
		Sections:    sections,
		Stats:       agg,
		GeneratedAt: time.Now(),
	}

	// Generate HTML report.
	if htmlPath, err := GenerateHTML(cfg.WorkspaceRoot, report); err == nil {
		report.HTMLPath = htmlPath
	}

	progress <- ProgressEvent{
		Phase:  "done",
		Detail: "Report ready",
		Pct:    1.0,
		Report: report,
	}
}

// FormatReport renders the report as markdown for display in the TUI.
func FormatReport(r *Report) string {
	var b strings.Builder

	b.WriteString("# Session Insights\n\n")
	b.WriteString(fmt.Sprintf("_Generated %s · %d sessions · %d messages · %.1f hours_\n\n",
		r.GeneratedAt.Format("2006-01-02 15:04"),
		r.Stats.TotalSessions,
		r.Stats.TotalMessages,
		r.Stats.TotalDurationH,
	))

	// HTML report link.
	if r.HTMLPath != "" {
		b.WriteString(fmt.Sprintf("**Full report:** `%s`\n\n", r.HTMLPath))
	}

	// At a Glance.
	b.WriteString("## At a Glance\n\n")
	if r.AtAGlance.WhatsWorking != "" {
		b.WriteString("**What's working:** " + r.AtAGlance.WhatsWorking + "\n\n")
	}
	if r.AtAGlance.WhatsHindering != "" {
		b.WriteString("**What's hindering you:** " + r.AtAGlance.WhatsHindering + "\n\n")
	}
	if r.AtAGlance.QuickWins != "" {
		b.WriteString("**Quick wins to try:** " + r.AtAGlance.QuickWins + "\n\n")
	}
	if r.AtAGlance.AmbitiousWorkflows != "" {
		b.WriteString("**Ambitious workflows:** " + r.AtAGlance.AmbitiousWorkflows + "\n\n")
	}

	// Sections.
	for _, s := range r.Sections {
		b.WriteString("## " + s.Title + "\n\n")
		b.WriteString(s.Content + "\n\n")
	}

	return b.String()
}

func send(ch chan<- ProgressEvent, phase, detail string, pct float64) {
	ch <- ProgressEvent{Phase: phase, Detail: detail, Pct: pct}
}

func sendErr(ch chan<- ProgressEvent, err error) {
	ch <- ProgressEvent{Phase: "error", Detail: err.Error(), Err: err}
}
