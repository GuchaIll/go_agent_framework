package core

import (
	"encoding/json"
	"fmt"
	"sync"
)

// SkillStepKind identifies what a skill step does.
type SkillStepKind string

const (
	KindLLM      SkillStepKind = "llm"
	KindTool     SkillStepKind = "tool"
	KindRAG      SkillStepKind = "rag"
	KindSubSkill SkillStepKind = "sub_skill"
)

// SkillStep is one node inside a Skill's DAG.
type SkillStep struct {
	Name      string          `json:"name"       yaml:"name"`
	Kind      SkillStepKind   `json:"kind"       yaml:"kind"`
	Config    json.RawMessage `json:"config,omitempty" yaml:"config,omitempty"`
	DependsOn []string        `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
}

// Skill is a reusable, composable workflow definition.
type Skill struct {
	Name        string      `json:"name"        yaml:"name"`
	Description string      `json:"description" yaml:"description"`
	Steps       []SkillStep `json:"steps"       yaml:"steps"`
}

// SkillRegistry stores available skills.
type SkillRegistry struct {
	mu     sync.RWMutex
	skills map[string]*Skill
}

// NewSkillRegistry creates an empty registry.
func NewSkillRegistry() *SkillRegistry {
	return &SkillRegistry{skills: make(map[string]*Skill)}
}

// Register adds a skill. Returns an error on duplicate names.
func (r *SkillRegistry) Register(s *Skill) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.skills[s.Name]; exists {
		return fmt.Errorf("skill %q already registered", s.Name)
	}
	r.skills[s.Name] = s
	return nil
}

// Get returns a skill by name.
func (r *SkillRegistry) Get(name string) (*Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.skills[name]
	return s, ok
}

// List returns a snapshot of all registered skill names.
func (r *SkillRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.skills))
	for n := range r.skills {
		names = append(names, n)
	}
	return names
}

// SkillExecutor runs a Skill by building a DAG from its steps and
// dispatching each step to the appropriate handler (LLM / Tool / RAG /
// sub-skill).
type SkillExecutor struct {
	Tools  *ToolRegistry
	Skills *SkillRegistry
}

// Execute runs the named skill's DAG within the given context.
func (e *SkillExecutor) Execute(skill *Skill, ctx *Context) error {
	dag := NewDAG()
	for _, ss := range skill.Steps {
		agent, err := e.agentFor(ss)
		if err != nil {
			return fmt.Errorf("skill %q step %q: %w", skill.Name, ss.Name, err)
		}
		if err := dag.Add(DAGStep{
			Name:      ss.Name,
			Agent:     agent,
			DependsOn: ss.DependsOn,
		}); err != nil {
			return err
		}
	}
	return dag.Run(ctx)
}

// agentFor returns a thin Agent adapter for the given skill step kind.
func (e *SkillExecutor) agentFor(ss SkillStep) (Agent, error) {
	switch ss.Kind {
	case KindTool:
		return &skillToolStep{name: ss.Name, config: ss.Config, registry: e.Tools}, nil
	case KindRAG:
		return &skillRAGStep{name: ss.Name}, nil
	case KindLLM:
		return &skillLLMStep{name: ss.Name}, nil
	case KindSubSkill:
		return &skillSubStep{name: ss.Name, config: ss.Config, executor: e}, nil
	default:
		return nil, fmt.Errorf("unknown step kind %q", ss.Kind)
	}
}

// ----- tiny adapters (each reads/writes from ctx.State) -----

// skillToolStep executes a tool. It reads the tool name from Config and
// arguments from ctx.State["tool_args_<stepName>"].
type skillToolStep struct {
	name     string
	config   json.RawMessage
	registry *ToolRegistry
}

func (s *skillToolStep) Name() string { return s.name }

func (s *skillToolStep) Run(ctx *Context) error {
	var cfg struct {
		Tool string `json:"tool"`
	}
	if err := json.Unmarshal(s.config, &cfg); err != nil {
		return fmt.Errorf("skillToolStep %q: bad config: %w", s.name, err)
	}

	argsKey := "tool_args_" + s.name
	args, _ := ctx.State[argsKey].(json.RawMessage)
	if args == nil {
		args = s.config // fall back to full config
	}

	result := s.registry.ExecuteTool(ctx.ToContext(), ToolCall{
		Name: cfg.Tool,
		Args: args,
	})
	ctx.State["tool_result_"+s.name] = result
	return nil
}

// skillRAGStep delegates to any RAGAgent already in ctx.State.
type skillRAGStep struct{ name string }

func (s *skillRAGStep) Name() string { return s.name }

func (s *skillRAGStep) Run(ctx *Context) error {
	// Assumes rag_query is already in state.
	ctx.Logger.Info("skill rag step", "step", s.name)
	return nil
}

// skillLLMStep is a pass-through; the caller is expected to have
// wired an LLM agent upstream.
type skillLLMStep struct{ name string }

func (s *skillLLMStep) Name() string { return s.name }

func (s *skillLLMStep) Run(ctx *Context) error {
	ctx.Logger.Info("skill llm step", "step", s.name)
	return nil
}

// skillSubStep runs a nested skill.
type skillSubStep struct {
	name     string
	config   json.RawMessage
	executor *SkillExecutor
}

func (s *skillSubStep) Name() string { return s.name }

func (s *skillSubStep) Run(ctx *Context) error {
	var cfg struct {
		Skill string `json:"skill"`
	}
	if err := json.Unmarshal(s.config, &cfg); err != nil {
		return fmt.Errorf("skillSubStep %q: bad config: %w", s.name, err)
	}

	skill, ok := s.executor.Skills.Get(cfg.Skill)
	if !ok {
		return fmt.Errorf("skillSubStep %q: unknown skill %q", s.name, cfg.Skill)
	}
	return s.executor.Execute(skill, ctx)
}

// SkillAgent is a graph-compatible Agent that runs a named skill.
// It reads the skill name from the SkillName field.
type SkillAgent struct {
	SkillName string
	Executor  *SkillExecutor
}

func (a *SkillAgent) Name() string { return "skill:" + a.SkillName }

func (a *SkillAgent) Run(ctx *Context) error {
	skill, ok := a.Executor.Skills.Get(a.SkillName)
	if !ok {
		return fmt.Errorf("skill_agent: unknown skill %q", a.SkillName)
	}
	return a.Executor.Execute(skill, ctx)
}
