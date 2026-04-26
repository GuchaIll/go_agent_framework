import { Handle, Position } from '@xyflow/react';

const BADGE_COLORS = {
  tools: '#3a7bd5',
  skills: '#8e44ad',
  rag: '#27ae60',
  agents: '#e67e22',
};

const STATUS_BG = {
  idle: '#2a2d3a',
  running: '#ffffff',
  success: '#2a2d3a',
  error: '#5c2020',
};

const STATUS_TEXT = {
  idle: '#e0e0e0',
  running: '#1a1d27',
  success: '#e0e0e0',
  error: '#e0e0e0',
};

export default function AgentNode({ data }) {
  const { label, description, tools, skills, rag, agents, model, status, subProcess, track } =
    data;
  const bg = STATUS_BG[status] || STATUS_BG.idle;
  const fg = STATUS_TEXT[status] || STATUS_TEXT.idle;
  const isRunning = status === 'running';

  return (
    <div style={{ ...styles.node, background: bg, color: fg }}>
      <Handle type="target" position={Position.Top} style={styles.handle} />

      <div style={styles.header}>
        <span style={styles.name}>{label}</span>
        {isRunning && <span style={styles.pulse} />}
      </div>

      {description && (
        <div style={{ ...styles.desc, opacity: isRunning ? 0.7 : 0.6 }}>
          {description}
        </div>
      )}

      <div style={styles.badges}>
        {model && (
          <span style={{ ...styles.badge, background: '#0ea5e9', color: '#fff' }}>
            {model === 'analysis' ? '🧠 analysis' : '⚡ orchestration'}
          </span>
        )}
        {track && (
          <span style={{ ...styles.badge, background: track === 'fast' ? '#16a34a' : '#ea580c', color: '#fff' }}>
            {track} track
          </span>
        )}
        {renderBadges('tools', tools, fg)}
        {renderBadges('skills', skills, fg)}
        {renderBadges('rag', rag, fg)}
        {renderBadges('agents', agents, fg)}
      </div>

      {subProcess && (
        <div
          style={{
            ...styles.subprocess,
            background: BADGE_COLORS[subProcess.kind] || '#555',
          }}
        >
          <div>{subProcess.kind}: {subProcess.name}</div>
          {subProcess.collection && (
            <div style={styles.subprocessMeta}>collection: {subProcess.collection}</div>
          )}
          {subProcess.description && (
            <div style={styles.subprocessMeta}>{subProcess.description}</div>
          )}
        </div>
      )}

      <Handle type="source" position={Position.Bottom} style={styles.handle} />
    </div>
  );
}

function renderBadges(kind, items, fg) {
  if (!items || items.length === 0) return null;
  return items.map((item) => (
    <span
      key={`${kind}-${item}`}
      style={{
        ...styles.badge,
        background: BADGE_COLORS[kind],
        color: '#fff',
      }}
    >
      {item}
    </span>
  ));
}

const styles = {
  node: {
    padding: '12px 16px',
    borderRadius: 10,
    border: '1px solid #3a3d4a',
    minWidth: 200,
    maxWidth: 280,
    fontFamily: 'inherit',
    transition: 'background 0.3s ease, color 0.3s ease',
  },
  header: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginBottom: 4,
  },
  name: {
    fontSize: 14,
    fontWeight: 700,
  },
  pulse: {
    width: 8,
    height: 8,
    borderRadius: '50%',
    backgroundColor: '#27ae60',
    animation: 'pulse 1.2s ease-in-out infinite',
  },
  desc: {
    fontSize: 12,
    lineHeight: 1.4,
    marginBottom: 6,
  },
  badges: {
    display: 'flex',
    flexWrap: 'wrap',
    gap: 4,
  },
  badge: {
    fontSize: 10,
    padding: '2px 6px',
    borderRadius: 4,
    fontWeight: 600,
    letterSpacing: '0.3px',
  },
  subprocess: {
    marginTop: 8,
    padding: '4px 8px',
    borderRadius: 6,
    fontSize: 11,
    color: '#fff',
    fontWeight: 600,
    animation: 'fadeIn 0.3s ease',
  },
  subprocessMeta: {
    marginTop: 4,
    fontSize: 10,
    fontWeight: 400,
    lineHeight: 1.35,
    opacity: 0.95,
  },
  handle: {
    width: 8,
    height: 8,
    background: '#555',
    border: '2px solid #3a3d4a',
  },
};
