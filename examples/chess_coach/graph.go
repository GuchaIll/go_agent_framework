package chess

import (
	"go_agent_framework/contrib/llm"
	"go_agent_framework/core"
	"go_agent_framework/examples/chess_coach/agents"
)

// BuildGraph wires the chess-coach pipeline:
//
//	serial(ingest) -> serial(inspection) -> parallel(analysis, strategy, guard) -> serial(feedback)
func BuildGraph(tools *core.ToolRegistry, skills *core.SkillRegistry, models llm.Models) *core.Graph {
	return core.NewGraph("chess_coach").
		AddSerial(&agents.IngestAgent{}).
		AddSerial(&agents.InspectionAgent{Tools: tools}).
		AddParallel(
			&agents.AnalysisAgent{Tools: tools, Depth: 20},
			&agents.StrategyAgent{LLM: models.For(llm.RoleAnalysis), Skills: skills, ModelRole: string(llm.RoleAnalysis)},
			&agents.GuardAgent{Tools: tools},
		).
		AddSerial(&agents.FeedbackAgent{})
}
