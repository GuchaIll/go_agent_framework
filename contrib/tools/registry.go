package tools

import "go_agent_framework/core"

// RegisterAll adds all contrib tools to the given registry.
func RegisterAll(reg *core.ToolRegistry) error {
	contribTools := []core.Tool{
		&Calculator{},
		&Weather{},
		&WebSearch{},
		&DatabaseQuery{},
	}
	for _, t := range contribTools {
		if err := reg.Register(t); err != nil {
			return err
		}
	}
	return nil
}
