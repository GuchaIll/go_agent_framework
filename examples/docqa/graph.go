package docqa

import (
	"go_agent_framework/contrib/llm"
	"go_agent_framework/contrib/vector"
	"go_agent_framework/core"
	"go_agent_framework/examples/docqa/agents"
)

// BuildGraph wires the doc-qa pipeline:
//
//	serial(load_doc) -> serial(chunk) -> serial(embed) -> serial(retrieve) -> serial(answer)
func BuildGraph(embedder agents.EmbeddingService, vdb vector.VectorDB, models llm.Models) *core.Graph {
	return core.NewGraph("doc_qa").
		AddSerial(&agents.LoadDocAgent{}).
		AddSerial(&agents.ChunkAgent{ChunkSize: 512, Overlap: 64}).
		AddSerial(&agents.EmbedAgent{Embedder: embedder, VectorDB: vdb}).
		AddSerial(&agents.RetrieveAgent{Embedder: embedder, VectorDB: vdb, TopK: 3}).
		AddSerial(&agents.AnswerAgent{LLM: models.For(llm.RoleAnalysis), ModelRole: string(llm.RoleAnalysis)})
}
