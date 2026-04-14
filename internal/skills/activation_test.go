package skills

import "testing"

func TestFilterActiveSkills_Unconditional(t *testing.T) {
	all := []Skill{
		{Name: "always", Paths: nil},
		{Name: "conditional", Paths: []string{"src/**/*.ts"}},
	}
	result := FilterActiveSkills(all, nil)
	if len(result) != 1 || result[0].Name != "always" {
		t.Errorf("expected only unconditional skill, got %v", names(result))
	}
}

func TestFilterActiveSkills_PathMatch(t *testing.T) {
	all := []Skill{
		{Name: "ts-skill", Paths: []string{"src/**/*.ts"}},
		{Name: "go-skill", Paths: []string{"**/*.go"}},
	}
	result := FilterActiveSkills(all, []string{"src/components/app.ts"})
	if len(result) != 1 || result[0].Name != "ts-skill" {
		t.Errorf("expected ts-skill, got %v", names(result))
	}
}

func TestFilterActiveSkills_NoMatch(t *testing.T) {
	all := []Skill{
		{Name: "py-skill", Paths: []string{"**/*.py"}},
	}
	result := FilterActiveSkills(all, []string{"src/main.go"})
	if len(result) != 0 {
		t.Errorf("expected no matches, got %v", names(result))
	}
}

func TestFilterActiveSkills_MultipleGlobs(t *testing.T) {
	all := []Skill{
		{Name: "web", Paths: []string{"**/*.ts", "**/*.tsx", "**/*.css"}},
	}
	result := FilterActiveSkills(all, []string{"app.css"})
	if len(result) != 1 {
		t.Errorf("expected match on .css glob, got %v", names(result))
	}
}

func names(skills []Skill) []string {
	out := make([]string, len(skills))
	for i, s := range skills {
		out[i] = s.Name
	}
	return out
}
