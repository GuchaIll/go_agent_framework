package observability

// PublishThought emits a chain-of-thought event for the current agent.
func PublishThought(graph, agent, session, message string) {
	DefaultHub().Publish(DashboardEvent{
		Type:    EventThought,
		Graph:   graph,
		Agent:   agent,
		Session: session,
		Message: message,
	})
}

// PublishToolCall emits a tool_call event when an agent invokes a tool.
func PublishToolCall(graph, agent, session, toolName string, args interface{}) {
	detail := map[string]interface{}{"tool": toolName}
	if args != nil {
		detail["args"] = args
	}
	DefaultHub().Publish(DashboardEvent{
		Type:    EventToolCall,
		Graph:   graph,
		Agent:   agent,
		Session: session,
		Message: "Calling tool: " + toolName,
		Detail:  detail,
	})
}

// PublishToolResult emits a tool_result event after tool execution.
func PublishToolResult(graph, agent, session, toolName, output, errMsg string) {
	detail := map[string]interface{}{"tool": toolName, "output": output}
	if errMsg != "" {
		detail["error"] = errMsg
	}
	msg := "Tool result: " + toolName
	if errMsg != "" {
		msg += " (error)"
	}
	DefaultHub().Publish(DashboardEvent{
		Type:    EventToolResult,
		Graph:   graph,
		Agent:   agent,
		Session: session,
		Message: msg,
		Detail:  detail,
	})
}

// PublishSkillUse emits a skill_use event when an agent uses a skill.
func PublishSkillUse(graph, agent, session, skillName, message string) {
	DefaultHub().Publish(DashboardEvent{
		Type:    EventSkillUse,
		Graph:   graph,
		Agent:   agent,
		Session: session,
		Message: message,
		Detail:  map[string]interface{}{"skill": skillName},
	})
}

// PublishDelegation emits a delegation event when work is passed to a sub-agent.
func PublishDelegation(graph, agent, session, targetAgent, reason string) {
	DefaultHub().Publish(DashboardEvent{
		Type:    EventDelegation,
		Graph:   graph,
		Agent:   agent,
		Session: session,
		Message: reason,
		Detail:  map[string]interface{}{"target": targetAgent},
	})
}

// PublishChatResponse emits the final chat response for display.
func PublishChatResponse(graph, session, message string) {
	DefaultHub().Publish(DashboardEvent{
		Type:    EventChatResponse,
		Graph:   graph,
		Session: session,
		Message: message,
	})
}
