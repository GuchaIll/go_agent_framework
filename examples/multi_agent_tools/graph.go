package multiagent

import (
	"go_agent_framework/contrib/llm"
	"go_agent_framework/core"
	"go_agent_framework/examples/multi_agent_tools/agents"
)

// BuildToolSkillGraph creates the multi-agent pipeline:
//
//	serial(planner) -> parallel(tool_executor, rag_retriever) -> serial(rag_answer)
func BuildToolSkillGraph(
	models llm.Models,
	toolReg *core.ToolRegistry,
	retriever core.Retriever,
) *core.Graph {
	return core.NewGraph("multi_agent_tools").
		AddSerial(&agents.PlannerAgent{LLM: models.For(llm.RoleOrchestration), Tools: toolReg, ModelRole: string(llm.RoleOrchestration)}).
		AddParallel(
			&agents.ToolExecutorAgent{Registry: toolReg},
			&core.RAGAgent{Retriever: retriever, TopK: 5},
		).
		AddSerial(&agents.RAGAnswerAgent{LLM: models.For(llm.RoleAnalysis), ModelRole: string(llm.RoleAnalysis)})
}
