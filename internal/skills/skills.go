package skills

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Skill represents a discovered skill definition.
type Skill struct {
	Name        string // canonical name without leading slash, e.g. "commit"
	Description string
	Content     string // full markdown body after frontmatter
	Source      string // "project" or "user"
	Path        string // filesystem path to the SKILL.md file
}

// Discover scans the given directories for skills and returns a deduplicated
// list. Project skills override user skills with the same name.
//
// Each directory is scanned for two formats:
//  1. Directory format: <dir>/<skill-name>/SKILL.md (preferred, CC-compatible)
//  2. Flat file format: <dir>/<skill-name>.md (legacy, simpler)
func Discover(projectDir, userDir string) []Skill {
	userSkills := scanDir(userDir, "user")
	projectSkills := scanDir(projectDir, "project")

	// Project overrides user (project is more specific).
	byName := make(map[string]Skill, len(projectSkills)+len(userSkills))
	for _, s := range userSkills {
		byName[s.Name] = s
	}
	for _, s := range projectSkills {
		byName[s.Name] = s
	}

	result := make([]Skill, 0, len(byName))
	for _, s := range byName {
		result = append(result, s)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// Find returns the skill with the given name (slash-prefix tolerated), or
// false if not found.
func Find(skills []Skill, name string) (Skill, bool) {
	name = strings.TrimPrefix(strings.TrimSpace(name), "/")
	for _, s := range skills {
		if s.Name == name {
			return s, true
		}
	}
	return Skill{}, false
}

func scanDir(dir, source string) []Skill {
	if dir == "" {
		return nil
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil
	}

	var skills []Skill
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())

		if entry.IsDir() {
			// Directory format: <dir>/<skill-name>/SKILL.md
			skillFile := findSkillMD(path)
			if skillFile == "" {
				continue
			}
			skill, parseErr := parseSkillFile(skillFile, source)
			if parseErr != nil {
				continue
			}
			// Use directory name as canonical name if frontmatter didn't provide one.
			if skill.Name == "" || skill.Name == strings.TrimSuffix(filepath.Base(skillFile), filepath.Ext(skillFile)) {
				skill.Name = entry.Name()
			}
			skill.Name = canonicalName(skill.Name)
			skills = append(skills, skill)
			continue
		}

		// Flat file format: <dir>/<skill-name>.md
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			continue
		}
		skill, parseErr := parseSkillFile(path, source)
		if parseErr != nil {
			continue
		}
		skill.Name = canonicalName(skill.Name)
		skills = append(skills, skill)
	}
	return skills
}

// findSkillMD returns the path to SKILL.md (case-insensitive) inside dir,
// or empty string if not found.
func findSkillMD(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.EqualFold(e.Name(), "SKILL.md") {
			return filepath.Join(dir, e.Name())
		}
	}
	return ""
}

// canonicalName strips leading slash and lowercases.
func canonicalName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimPrefix(name, "/")
	return name
}

func parseSkillFile(path, source string) (Skill, error) {
	f, err := os.Open(path)
	if err != nil {
		return Skill{}, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4096), 1024*1024)

	// Check for frontmatter start.
	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "---" {
		return Skill{}, fmt.Errorf("no frontmatter")
	}

	// Parse frontmatter (simple key:value pairs, ignore complex YAML).
	var name, description string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			break
		}
		if k, v, ok := splitYAMLLine(line); ok {
			switch k {
			case "name":
				name = v
			case "description":
				description = v
			}
		}
	}

	if name == "" {
		// Use filename (without extension) as fallback.
		base := filepath.Base(path)
		// For SKILL.md inside a directory, use the parent directory name.
		if strings.EqualFold(base, "SKILL.md") {
			name = filepath.Base(filepath.Dir(path))
		} else {
			name = strings.TrimSuffix(base, filepath.Ext(base))
		}
	}

	// Read body.
	var body strings.Builder
	for scanner.Scan() {
		if body.Len() > 0 {
			body.WriteString("\n")
		}
		body.WriteString(scanner.Text())
	}

	return Skill{
		Name:        name,
		Description: description,
		Content:     body.String(),
		Source:      source,
		Path:        path,
	}, nil
}

// splitYAMLLine parses a "key: value" YAML line, stripping quotes.
func splitYAMLLine(line string) (key, value string, ok bool) {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:idx])
	value = strings.TrimSpace(line[idx+1:])
	value = strings.Trim(value, `"'`)
	return key, value, key != ""
}
