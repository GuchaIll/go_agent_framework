import { useState, useRef, useEffect } from 'react';

const EVENT_ICONS = {
  thought: '🤔',
  tool_call: '🔧',
  tool_result: '⚡',
  skill_use: '✨',
  delegation: '🔀',
};

function StepBubble({ step }) {
  const icon = EVENT_ICONS[step.type] || '💬';
  const bg = {
    thought: '#1e293b',
    tool_call: '#1a2744',
    tool_result: '#172530',
    skill_use: '#261a35',
    delegation: '#2a2217',
  }[step.type] || '#1e293b';

  const label = {
    thought: 'Thought',
    tool_call: 'Tool Call',
    tool_result: 'Tool Result',
    skill_use: 'Skill Use',
    delegation: 'Delegation',
  }[step.type] || step.type;

  return (
    <div style={{
      background: bg,
      border: '1px solid #2a2d3a',
      borderRadius: 8,
      padding: '6px 10px',
      marginBottom: 4,
      fontSize: 12,
      lineHeight: 1.4,
    }}>
      <span style={{ marginRight: 6 }}>{icon}</span>
      <span style={{ color: '#6b7280', fontWeight: 600 }}>[{step.agent}] {label}</span>
      {step.detail?.tool && (
        <span style={{ color: '#60a5fa', marginLeft: 6 }}>{step.detail.tool}</span>
      )}
      {step.detail?.skill && (
        <span style={{ color: '#a78bfa', marginLeft: 6 }}>{step.detail.skill}</span>
      )}
      {step.detail?.target_agent && (
        <span style={{ color: '#fbbf24', marginLeft: 6 }}>→ {step.detail.target_agent}</span>
      )}
      {step.message && (
        <div style={{ color: '#9ca3af', marginTop: 2 }}>{step.message}</div>
      )}
    </div>
  );
}

export default function ChatRoom({ eventSourceRef }) {
  const [messages, setMessages] = useState([]);
  const [input, setInput] = useState('');
  const [sending, setSending] = useState(false);
  const bottomRef = useRef(null);
  const sessionRef = useRef(null);

  // Listen for chain-of-thought events from the shared SSE connection.
  useEffect(() => {
    const handler = (msg) => {
      try {
        const evt = JSON.parse(msg.data);
        const cotTypes = ['thought', 'tool_call', 'tool_result', 'skill_use', 'delegation'];
        if (cotTypes.includes(evt.type)) {
          setMessages((prev) => {
            const last = prev[prev.length - 1];
            if (last && last.role === 'assistant' && !last.final) {
              return [
                ...prev.slice(0, -1),
                { ...last, steps: [...last.steps, evt] },
              ];
            }
            return prev;
          });
        }
        if (evt.type === 'chat_response') {
          setMessages((prev) => {
            const last = prev[prev.length - 1];
            if (last && last.role === 'assistant' && !last.final) {
              return [
                ...prev.slice(0, -1),
                { ...last, text: evt.message, final: true },
              ];
            }
            return [...prev, { role: 'assistant', text: evt.message, steps: [], final: true }];
          });
          setSending(false);
        }
      } catch { /* ignore */ }
    };

    const es = eventSourceRef?.current;
    if (es) {
      es.addEventListener('message', handler);
      return () => es.removeEventListener('message', handler);
    }
  }, [eventSourceRef]);

  // Auto-scroll to bottom.
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  const send = async () => {
    const text = input.trim();
    if (!text || sending) return;

    setInput('');
    setSending(true);

    // Add user message and a placeholder assistant message.
    setMessages((prev) => [
      ...prev,
      { role: 'user', text },
      { role: 'assistant', text: '', steps: [], final: false },
    ]);

    try {
      const res = await fetch('/dashboard/chat', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ message: text, session_id: sessionRef.current || '' }),
      });
      const data = await res.json();
      sessionRef.current = data.session_id;

      // If the SSE chat_response hasn't arrived yet, finalize from HTTP response.
      setMessages((prev) => {
        const last = prev[prev.length - 1];
        if (last && last.role === 'assistant' && !last.final) {
          return [
            ...prev.slice(0, -1),
            { ...last, text: data.response || data.error || 'No response', final: true },
          ];
        }
        return prev;
      });
      setSending(false);
    } catch (err) {
      setMessages((prev) => {
        const last = prev[prev.length - 1];
        if (last && last.role === 'assistant' && !last.final) {
          return [
            ...prev.slice(0, -1),
            { ...last, text: `Error: ${err.message}`, final: true },
          ];
        }
        return prev;
      });
      setSending(false);
    }
  };

  const handleKey = (e) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      send();
    }
  };

  return (
    <div style={{
      width: 400,
      display: 'flex',
      flexDirection: 'column',
      borderLeft: '1px solid #2a2d3a',
      background: '#0f1117',
    }}>
      {/* Header */}
      <div style={{
        padding: '12px 16px',
        borderBottom: '1px solid #2a2d3a',
        fontWeight: 700,
        fontSize: 14,
        color: '#e0e0e0',
        display: 'flex',
        alignItems: 'center',
        gap: 8,
      }}>
        💬 Chat Room
        {sending && <span style={{ fontSize: 11, color: '#6b7280', fontWeight: 400 }}>processing…</span>}
      </div>

      {/* Messages */}
      <div style={{
        flex: 1,
        overflowY: 'auto',
        padding: '12px 12px 4px',
      }}>
        {messages.length === 0 && (
          <div style={{ color: '#4b5563', textAlign: 'center', marginTop: 40, fontSize: 13 }}>
            Send a message to test the agent pipeline.
          </div>
        )}
        {messages.map((msg, i) => (
          <div key={i} style={{
            marginBottom: 12,
            display: 'flex',
            flexDirection: 'column',
            alignItems: msg.role === 'user' ? 'flex-end' : 'flex-start',
          }}>
            {/* Role label */}
            <span style={{
              fontSize: 10,
              color: '#6b7280',
              marginBottom: 3,
              textTransform: 'uppercase',
              letterSpacing: '0.05em',
            }}>
              {msg.role === 'user' ? 'You' : 'Agent'}
            </span>

            {/* Chain-of-thought steps (assistant only) */}
            {msg.role === 'assistant' && msg.steps?.length > 0 && (
              <div style={{ width: '100%', marginBottom: 4 }}>
                {msg.steps.map((step, j) => (
                  <StepBubble key={j} step={step} />
                ))}
              </div>
            )}

            {/* Message text */}
            {(msg.text || msg.role === 'user') && (
              <div style={{
                background: msg.role === 'user' ? '#1d4ed8' : '#1e293b',
                borderRadius: 10,
                padding: '8px 12px',
                maxWidth: '90%',
                fontSize: 13,
                lineHeight: 1.5,
                whiteSpace: 'pre-wrap',
                wordBreak: 'break-word',
                border: msg.role === 'assistant' && msg.final ? '1px solid #334155' : 'none',
              }}>
                {msg.text || (msg.role === 'assistant' && !msg.final && (
                  <span style={{ color: '#6b7280' }}>Thinking…</span>
                ))}
              </div>
            )}
          </div>
        ))}
        <div ref={bottomRef} />
      </div>

      {/* Input */}
      <div style={{
        padding: '8px 12px 12px',
        borderTop: '1px solid #2a2d3a',
        display: 'flex',
        gap: 8,
      }}>
        <textarea
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={handleKey}
          placeholder="Type a message…"
          rows={1}
          style={{
            flex: 1,
            background: '#1a1d27',
            border: '1px solid #2a2d3a',
            borderRadius: 8,
            padding: '8px 12px',
            color: '#e0e0e0',
            fontSize: 13,
            resize: 'none',
            outline: 'none',
            fontFamily: 'inherit',
          }}
        />
        <button
          onClick={send}
          disabled={sending || !input.trim()}
          style={{
            background: sending || !input.trim() ? '#1a1d27' : '#1d4ed8',
            color: sending || !input.trim() ? '#4b5563' : '#ffffff',
            border: 'none',
            borderRadius: 8,
            padding: '8px 16px',
            cursor: sending || !input.trim() ? 'default' : 'pointer',
            fontWeight: 600,
            fontSize: 13,
          }}
        >
          Send
        </button>
      </div>
    </div>
  );
}
