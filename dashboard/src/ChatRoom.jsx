import { useEffect, useMemo, useRef, useState } from 'react';

const TOOL_COLLECTION = {
  get_opening_plan: 'openings',
  get_middlegame_theme: 'tactics',
  get_endgame_principle: 'endgames',
  get_general_advice: 'beginner_principles',
  explain_tactic: 'beginner_principles',
  explain_puzzle_objective: 'beginner_principles',
};

function newLine(evt, text, kind = evt.type) {
  return {
    id: `${kind}-${evt.agent || 'graph'}-${evt.ts}-${Math.random().toString(36).slice(2, 8)}`,
    ts: evt.ts,
    agent: evt.agent || 'system',
    kind,
    text,
  };
}

export default function ChatRoom({ eventSourceRef, pipelineTrack }) {
  const [lines, setLines] = useState([]);
  const bottomRef = useRef(null);
  const attachedRef = useRef(false);

  const trackLabel = useMemo(() => {
    if (pipelineTrack === 'fast') return 'Fast track';
    if (pipelineTrack === 'slow') return 'Slow track';
    return 'Track pending';
  }, [pipelineTrack]);

  useEffect(() => {
    const appendLine = (evt, text, kind = evt.type) => {
      setLines((prev) => [...prev, newLine(evt, text, kind)]);
    };

    const shortOutput = (value) => {
      if (!value) return '';
      const text = String(value).replace(/\s+/g, ' ').trim();
      if (text.length <= 120) return text;
      return `${text.slice(0, 117)}...`;
    };

    const handler = (msg) => {
      try {
        const evt = JSON.parse(msg.data);
        if (evt.type === 'graph_start') {
          setLines([]);
          appendLine(evt, 'Graph execution started.', 'graph');
          return;
        }
        if (evt.type === 'agent_start') {
          appendLine(evt, 'Entered state.', 'agent_start');
          return;
        }
        if (evt.type === 'agent_end') {
          appendLine(evt, `Exited state with status: ${evt.status || 'success'}.`, 'agent_end');
          return;
        }
        if (evt.type === 'thought') {
          appendLine(evt, evt.message || '', 'thought');
          return;
        }
        if (evt.type === 'tool_call') {
          const tool = evt.detail?.tool || 'tool';
          const collection = TOOL_COLLECTION[tool];
          appendLine(evt, `Tool call: ${tool}${collection ? ` [RAG: ${collection}]` : ''}`, 'tool_call');
          return;
        }
        if (evt.type === 'tool_result') {
          const tool = evt.detail?.tool || 'tool';
          const error = evt.detail?.error;
          const output = shortOutput(evt.detail?.output);
          appendLine(evt, error ? `Tool result: ${tool} failed: ${error}` : `Tool result: ${tool}${output ? ` -> ${output}` : ''}`, 'tool_result');
          return;
        }
        if (evt.type === 'skill_use') {
          appendLine(evt, `Skill: ${evt.detail?.skill || 'skill'} — ${evt.message || ''}`, 'skill_use');
          return;
        }
        if (evt.type === 'chat_response') {
          appendLine(evt, `Response emitted (${(evt.message || '').length} chars).`, 'chat_response');
          return;
        }
      } catch {
        // ignore malformed dashboard events
      }
    };

    let attached = null;
    const tryAttach = () => {
      const es = eventSourceRef?.current;
      if (!es || attachedRef.current) return;
      es.addEventListener('message', handler);
      attachedRef.current = true;
      attached = es;
    };

    tryAttach();
    const interval = window.setInterval(tryAttach, 100);
    return () => {
      window.clearInterval(interval);
      if (attached) {
        attached.removeEventListener('message', handler);
      }
      attachedRef.current = false;
    };
  }, [eventSourceRef]);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [lines]);

  return (
    <div style={{
      width: 400,
      display: 'flex',
      flexDirection: 'column',
      borderLeft: '1px solid #2a2d3a',
      background: '#0f1117',
    }}>
      <div style={{
        padding: '12px 16px',
        borderBottom: '1px solid #2a2d3a',
        fontWeight: 700,
        fontSize: 14,
        color: '#e0e0e0',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
      }}>
        <span>Execution Log</span>
        <span style={{
          fontSize: 11,
          fontWeight: 600,
          color: pipelineTrack === 'fast' ? '#86efac' : pipelineTrack === 'slow' ? '#fdba74' : '#9ca3af',
        }}>{trackLabel}</span>
      </div>

      <div style={{ flex: 1, overflowY: 'auto', padding: '12px' }}>
        {lines.length === 0 && (
          <div style={{ color: '#4b5563', textAlign: 'center', marginTop: 40, fontSize: 13 }}>
            Waiting for orchestration events.
          </div>
        )}
        {lines.map((line) => (
          <div key={line.id} style={{
            display: 'grid',
            gridTemplateColumns: '62px 96px 1fr',
            gap: 10,
            alignItems: 'start',
            padding: '8px 0',
            borderBottom: '1px solid rgba(42,45,58,0.7)',
            fontSize: 12,
            lineHeight: 1.45,
          }}>
            <div style={{ color: '#64748b', fontVariantNumeric: 'tabular-nums' }}>
              {new Date(line.ts || Date.now()).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
            </div>
            <div style={{ color: '#cbd5e1', fontWeight: 700, textTransform: 'lowercase' }}>
              {line.agent}
            </div>
            <div style={{ color: line.kind === 'tool_result' ? '#93c5fd' : line.kind === 'thought' ? '#e5e7eb' : '#cbd5e1' }}>
              {line.text}
            </div>
          </div>
        ))}
        <div ref={bottomRef} />
      </div>
    </div>
  );
}
