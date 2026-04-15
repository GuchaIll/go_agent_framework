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
  const eventSourceRef = useRef(null);

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
    (agentId, status, subProcess) => {
      setNodes((prev) =>
        prev.map((n) => {
          if (n.id !== agentId) return n;
          return {
            ...n,
            data: {
              ...n.data,
              status: status ?? n.data.status,
              subProcess: subProcess !== undefined ? subProcess : n.data.subProcess,
            },
          };
        })
      );
    },
    [setNodes]
  );

  useEffect(() => {
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
            setNodes((prev) =>
              prev.map((n) => ({
                ...n,
                data: { ...n.data, status: 'idle', subProcess: null },
              }))
            );
            break;
          default:
            break;
        }
      } catch {
        /* ignore malformed */
      }
    };

    return () => {
      es.close();
      setConnected(false);
    };
  }, [updateNodeStatus, setNodes]);

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
        <ChatRoom eventSourceRef={eventSourceRef} />
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
