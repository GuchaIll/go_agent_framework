package skills

import (
	"encoding/json"
	"fmt"
	"go_agent_framework/core"
	"os"
	"path/filepath"
	"strings"
)

// LoadFromDir reads all .json skill files in a directory and registers
// them into the given SkillRegistry.
func LoadFromDir(dir string, reg *core.SkillRegistry) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("skills: read dir %q: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
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
