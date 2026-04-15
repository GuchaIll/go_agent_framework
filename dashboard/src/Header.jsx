import { useState, useEffect } from 'react';

const POLL_INTERVAL = 2000;

export default function Header({ graphName, connected }) {
  const [stats, setStats] = useState({
    llm_requests_total: 0,
    llm_prompt_tokens: 0,
    llm_completion_tokens: 0,
    llm_estimated_tokens_total: 0,
    llm_estimated_cost_usd: 0,
  });

  useEffect(() => {
    let active = true;
    const poll = async () => {
      try {
        const res = await fetch('/dashboard/stats');
        if (res.ok && active) {
          setStats(await res.json());
        }
      } catch {
        /* ignore */
      }
    };
    poll();
    const id = setInterval(poll, POLL_INTERVAL);
    return () => {
      active = false;
      clearInterval(id);
    };
  }, []);

  return (
    <header style={styles.header}>
      <div style={styles.left}>
        <span style={styles.title}>{graphName || 'Agent Dashboard'}</span>
      </div>

      <div style={styles.stats}>
        <Stat label="LLM Calls" value={fmt(stats.llm_requests_total)} />
        <Stat label="Prompt Tokens" value={fmt(stats.llm_prompt_tokens)} />
        <Stat label="Completion Tokens" value={fmt(stats.llm_completion_tokens)} />
        <Stat label="Total Tokens" value={fmt(stats.llm_estimated_tokens_total)} />
        <Stat label="Est. Cost" value={`$${stats.llm_estimated_cost_usd.toFixed(4)}`} />
      </div>

      <div style={styles.right}>
        <span
          style={{
            ...styles.dot,
            backgroundColor: connected ? '#27ae60' : '#e74c3c',
          }}
        />
        <span style={styles.statusText}>
          {connected ? 'Connected' : 'Disconnected'}
        </span>
      </div>
    </header>
  );
}

function Stat({ label, value }) {
  return (
    <div style={styles.stat}>
      <span style={styles.statValue}>{value}</span>
      <span style={styles.statLabel}>{label}</span>
    </div>
  );
}

function fmt(n) {
  if (n == null) return '0';
  return Number(n).toLocaleString();
}

const styles = {
  header: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: '10px 20px',
    background: '#1a1d27',
    borderBottom: '1px solid #2a2d3a',
    height: 56,
    flexShrink: 0,
  },
  left: {
    display: 'flex',
    alignItems: 'center',
    gap: 12,
  },
  title: {
    fontSize: 16,
    fontWeight: 700,
    color: '#ffffff',
    letterSpacing: '0.5px',
  },
  stats: {
    display: 'flex',
    gap: 24,
  },
  stat: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    gap: 2,
  },
  statValue: {
    fontSize: 14,
    fontWeight: 600,
    color: '#e0e0e0',
  },
  statLabel: {
    fontSize: 10,
    color: '#888',
    textTransform: 'uppercase',
    letterSpacing: '0.5px',
  },
  right: {
    display: 'flex',
    alignItems: 'center',
    gap: 6,
  },
  dot: {
    width: 8,
    height: 8,
    borderRadius: '50%',
    display: 'inline-block',
  },
  statusText: {
    fontSize: 12,
    color: '#aaa',
  },
};
