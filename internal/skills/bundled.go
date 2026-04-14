package skills

import "embed"

//go:embed bundled/*.md
var bundledFS embed.FS

// BundledSkills returns skills compiled into the binary. These are
// parsed at call time from the embedded filesystem. Discovered skills
// with the same name take precedence (project customization wins).
func BundledSkills() []Skill {
	entries, err := bundledFS.ReadDir("bundled")
	if err != nil {
		return nil
	}
	var out []Skill
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := bundledFS.ReadFile("bundled/" + e.Name())
		if err != nil {
			continue
		}
		s := parseSkillContent(string(data), e.Name(), "bundled", "")
		if s.Name != "" {
			out = append(out, s)
		}
	}
	return out
}

// MergeWithBundled merges discovered skills with bundled ones.
// Discovered skills override bundled skills with the same name.
func MergeWithBundled(discovered []Skill) []Skill {
	bundled := BundledSkills()
	if len(bundled) == 0 {
		return discovered
	}

	// Index discovered names for dedup.
	seen := make(map[string]bool, len(discovered))
	for _, s := range discovered {
		seen[s.Name] = true
	}

	// Append bundled skills not overridden by discovered ones.
	merged := make([]Skill, len(discovered), len(discovered)+len(bundled))
	copy(merged, discovered)
	for _, s := range bundled {
		if !seen[s.Name] {
			merged = append(merged, s)
		}
	}
	return merged
}
