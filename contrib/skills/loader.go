package skills

import (
	"encoding/json"
	"fmt"
	"go_agent_framework/core"
	"os"
	"path/filepath"
	"strings"
)

// LoadFromDir reads skill definitions from a directory and registers them.
// It supports two layouts:
//
//  1. Flat: *.json files directly in dir (legacy).
//  2. Folder-per-skill: each subdirectory contains a skill.json (Anthropic convention).
//
// Both layouts can coexist in the same directory.
func LoadFromDir(dir string, reg *core.SkillRegistry) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("skills: read dir %q: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			// Folder-per-skill: look for skill.json inside the subdirectory.
			skillFile := filepath.Join(dir, e.Name(), "skill.json")
			if _, err := os.Stat(skillFile); err == nil {
				if err := loadFile(skillFile, reg); err != nil {
					return err
				}
			}
			continue
		}
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		if err := loadFile(path, reg); err != nil {
			return err
		}
	}
	return nil
}

func loadFile(path string, reg *core.SkillRegistry) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("skills: read %q: %w", path, err)
	}
	var skill core.Skill
	if err := json.Unmarshal(data, &skill); err != nil {
		return fmt.Errorf("skills: parse %q: %w", path, err)
	}
	return reg.Register(&skill)
}
