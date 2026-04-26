import { useState, useEffect, useCallback, useRef } from 'react';
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  useNodesState,
  useEdgesState,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import Dagre from '@dagrejs/dagre';
import Header from './Header';
import AgentNode from './AgentNode';
import ChatRoom from './ChatRoom';

const nodeTypes = { agent: AgentNode };
const TOOL_META = {
  analyze_position: { description: 'Runs engine evaluation for the current position.' },
  get_principal_variation: { description: 'Retrieves the best calculated continuation.' },
  detect_blunders: { description: 'Checks whether the submitted move loses significant value.' },
  is_move_legal: { description: 'Verifies that referenced moves are legal in the current position.' },
  get_opening_plan: { description: 'Retrieves opening guidance from the curated library.', collection: 'openings' },
  get_middlegame_theme: { description: 'Retrieves middlegame themes from the curated library.', collection: 'tactics' },
  get_endgame_principle: { description: 'Retrieves endgame guidance from the curated library.', collection: 'endgames' },
  get_general_advice: { description: 'Retrieves beginner coaching principles.', collection: 'beginner_principles' },
  explain_tactic: { description: 'Retrieves tactic explanations for the move/question.', collection: 'beginner_principles' },
  explain_puzzle_objective: { description: 'Retrieves puzzle-objective guidance.', collection: 'beginner_principles' },
};

// Layout nodes using dagre.
function layoutGraph(nodes, edges) {
  const g = new Dagre.graphlib.Graph().setDefaultEdgeLabel(() => ({}));
  g.setGraph({ rankdir: 'TB', nodesep: 80, ranksep: 100 });

  nodes.forEach((node) => {
    g.setNode(node.id, { width: 240, height: 120 });
  });
  edges.forEach((edge) => {
    g.setEdge(edge.source, edge.target);
  });

  Dagre.layout(g);

  return nodes.map((node) => {
    const pos = g.node(node.id);
    return {
      ...node,
      position: { x: pos.x - 120, y: pos.y - 60 },
    };
  });
}

function buildNodesAndEdges(graphInfo) {
  const nodes = (graphInfo.nodes || []).map((n) => ({
    id: n.id,
    type: 'agent',
    position: { x: 0, y: 0 },
    data: {
      label: n.id,
      description: n.description || '',
      tools: n.tools || [],
      skills: n.skills || [],
      rag: n.rag || [],
      agents: n.agents || [],
      model: n.model || '',
      status: 'idle',
      subProcess: null,
      track: null,
    },
  }));

  const edges = (graphInfo.edges || []).map((e, i) => ({
    id: `e-${e.source}-${e.target}-${i}`,
    source: e.source,
    target: e.target,
    animated: e.parallel,
    style: e.parallel
      ? { stroke: '#6c8ebf', strokeWidth: 2, strokeDasharray: '6 3' }
      : { stroke: '#b0b0b0', strokeWidth: 2 },
  }));

  const laid = layoutGraph(nodes, edges);
  return { nodes: laid, edges };
}

export default function App() {
  const [nodes, setNodes, onNodesChange] = useNodesState([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState([]);
  const [graphName, setGraphName] = useState('');
  const [connected, setConnected] = useState(false);
  const [pipelineTrack, setPipelineTrack] = useState('pending');
  const eventSourceRef = useRef(null);
  const subprocessTimersRef = useRef(new Map());
  const pipelineTrackRef = useRef('pending');
  pipelineTrackRef.current = pipelineTrack;

  const setPipelineTrackImmediate = useCallback((track) => {
    pipelineTrackRef.current = track;
    setPipelineTrack(track);
  }, []);

  // Fetch graph structure on mount.
  useEffect(() => {
    let active = true;
    (async () => {
      try {
        const res = await fetch('/dashboard/graph');
        if (!res.ok) return;
        const info = await res.json();
        if (!active) return;
        setGraphName(info.name || '');
        const { nodes: n, edges: e } = buildNodesAndEdges(info);
        setNodes(n);
        setEdges(e);
      } catch {
        /* retry on next mount */
      }
    })();
    return () => {
      active = false;
    };
  }, [setNodes, setEdges]);

  // SSE connection.
  const updateNodeStatus = useCallback(
    (agentId, status, subProcess, track) => {
      setNodes((prev) =>
        prev.map((n) => {
          if (n.id !== agentId) return n;
          return {
            ...n,
            data: {
              ...n.data,
              status: status ?? n.data.status,
              subProcess: subProcess !== undefined ? subProcess : n.data.subProcess,
              track: track !== undefined ? track : n.data.track,
            },
          };
        })
      );
    },
    [setNodes]
  );

  useEffect(() => {
    const showTimedSubprocess = (agent, subProcess) => {
      updateNodeStatus(agent, undefined, subProcess);
      const existing = subprocessTimersRef.current.get(agent);
      if (existing) clearTimeout(existing);
      const timeout = setTimeout(() => {
        updateNodeStatus(agent, undefined, null);
        subprocessTimersRef.current.delete(agent);
      }, 2000);
      subprocessTimersRef.current.set(agent, timeout);
    };

    const es = new EventSource('/dashboard/events');
    eventSourceRef.current = es;

    es.onopen = () => setConnected(true);
    es.onerror = () => setConnected(false);

    es.onmessage = (msg) => {
      try {
        const evt = JSON.parse(msg.data);
        switch (evt.type) {
          case 'agent_start':
            updateNodeStatus(evt.agent, 'running', null);
            break;
          case 'agent_end':
            updateNodeStatus(evt.agent, evt.status || 'success', null);
            if (evt.agent === 'coach' && pipelineTrackRef.current === 'pending') {
              setPipelineTrackImmediate('fast');
              updateNodeStatus(evt.agent, undefined, undefined, 'fast');
            }
            break;
          case 'tool_call': {
            const tool = evt.detail?.tool;
            const meta = TOOL_META[tool] || {};
            showTimedSubprocess(evt.agent, {
              name: tool,
              kind: 'tool',
              description: meta.description || evt.message,
              collection: meta.collection || null,
            });
            break;
          }
          case 'skill_use':
            showTimedSubprocess(evt.agent, {
              name: evt.detail?.skill,
              kind: 'skill',
              description: evt.message,
              collection: null,
            });
            break;
          case 'subprocess_start':
            updateNodeStatus(evt.agent, undefined, {
              name: evt.sub_process,
              kind: evt.sub_kind,
            });
            break;
          case 'subprocess_end':
            updateNodeStatus(evt.agent, undefined, null);
            break;
          case 'graph_start':
            // Reset all nodes to idle.
            setPipelineTrackImmediate('pending');
            setNodes((prev) =>
              prev.map((n) => ({
                ...n,
                data: { ...n.data, status: 'idle', subProcess: null, track: null },
              }))
            );
            break;
          case 'thought':
            if (evt.agent === 'coach' && typeof evt.message === 'string') {
              if (evt.message.includes('fast path')) {
                setPipelineTrackImmediate('fast');
                updateNodeStatus(evt.agent, undefined, undefined, 'fast');
              } else if (evt.message.includes('Coach triggered')) {
                setPipelineTrackImmediate('slow');
                updateNodeStatus(evt.agent, undefined, undefined, 'slow');
              }
            }
            break;
          default:
            break;
        }
      } catch {
        /* ignore malformed */
      }
    };

    return () => {
      subprocessTimersRef.current.forEach((timeout) => clearTimeout(timeout));
      subprocessTimersRef.current.clear();
      es.close();
      setConnected(false);
    };
  }, [setPipelineTrackImmediate, updateNodeStatus, setNodes]);

  return (
    <div style={{ width: '100vw', height: '100vh', display: 'flex', flexDirection: 'column' }}>
      <Header graphName={graphName} connected={connected} />
      <div style={{ flex: 1, display: 'flex', minHeight: 0 }}>
        <div style={{ flex: 1, position: 'relative' }}>
          <ReactFlow
            nodes={nodes}
            edges={edges}
            onNodesChange={onNodesChange}
            onEdgesChange={onEdgesChange}
            nodeTypes={nodeTypes}
            fitView
            proOptions={{ hideAttribution: true }}
            minZoom={0.3}
            maxZoom={2}
          >
            <Background color="#2a2d3a" gap={20} />
            <Controls />
            <MiniMap
              nodeColor={() => '#3a3d4a'}
              maskColor="rgba(0,0,0,0.6)"
            />
          </ReactFlow>
        </div>
        <ChatRoom eventSourceRef={eventSourceRef} pipelineTrack={pipelineTrack} />
      </div>

      {/* Keyframe animations */}
      <style>{`
        @keyframes pulse {
          0%, 100% { opacity: 1; transform: scale(1); }
          50% { opacity: 0.4; transform: scale(1.3); }
        }
        @keyframes fadeIn {
          from { opacity: 0; transform: translateY(-4px); }
          to { opacity: 1; transform: translateY(0); }
        }
      `}</style>
    </div>
  );
}
