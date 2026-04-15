package agents

import (
	"fmt"
	"go_agent_framework/core"
	"go_agent_framework/observability"
)

// LoadDocAgent reads a raw document from State["document"] and stores it
// for downstream chunking. In a production system this would fetch from
// S3 / GCS / a URL.
type LoadDocAgent struct{}

func (a *LoadDocAgent) Name() string        { return "load_doc" }
func (a *LoadDocAgent) Description() string { return "Reads raw document from input for downstream chunking." }
func (a *LoadDocAgent) Capabilities() core.AgentCapabilities { return core.AgentCapabilities{} }

func (a *LoadDocAgent) Run(ctx *core.Context) error {
	doc, _ := ctx.State["document"].(string)
	if doc == "" {
		return fmt.Errorf("load_doc: document is required")
	}
	observability.PublishThought(ctx.GraphName, a.Name(), ctx.SessionID, fmt.Sprintf("Loaded document (%d chars).", len(doc)))
	ctx.State["raw_document"] = doc
	ctx.Logger.Info("load_doc complete", "length", len(doc))
	return nil
}
