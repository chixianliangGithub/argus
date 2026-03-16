import React, { useCallback, useEffect } from 'react';
import ReactFlow, {
  Controls,
  Background,
  useNodesState,
  useEdgesState,
  addEdge,
  type Connection,
  type Edge,
  type Node,
  BackgroundVariant
} from 'reactflow';
import 'reactflow/dist/style.css';
import { useNavigate } from 'react-router-dom';
import { getServicesOverview } from '../services/service';

const statusColor = (s: string) => {
  if (s === 'down') return '#ff4d4f';
  if (s === 'degraded') return '#faad14';
  return '#52c41a';
};

const TopologyView: React.FC = () => {
  const navigate = useNavigate();
  const [nodes, setNodes, onNodesChange] = useNodesState<Node[]>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge[]>([]);

  useEffect(() => {
    (async () => {
      try {
        const res: any = await getServicesOverview({ window: '24h' });
        const services: any[] = Array.isArray(res?.services) ? res.services : [];
        const cols = 4;
        const w = 210;
        const h = 90;
        const next: Node[] = services.slice(0, 16).map((s, idx) => {
          const x = (idx % cols) * w;
          const y = Math.floor(idx / cols) * h;
          const c = statusColor(String(s.status || ''));
          return {
            id: String(s.service),
            data: { label: s.service },
            position: { x, y },
            style: { background: '#1f1f1f', color: c, border: `1px solid ${c}`, width: 180 },
          };
        });
        setNodes(next);
        setEdges([]);
      } catch (e) {
        setNodes([]);
        setEdges([]);
      }
    })();
  }, [setNodes, setEdges]);

  const onConnect = useCallback(
    (params: Connection) => setEdges((eds) => addEdge(params, eds)),
    [setEdges],
  );

  const onNodeClick = useCallback(
    (_: any, node: Node) => {
      const svc = String((node as any)?.id || '').trim();
      if (!svc) return;
      navigate(`/services?selected=${encodeURIComponent(svc)}`);
    },
    [navigate]
  );

  return (
    <div className="w-full h-[400px] glass-panel rounded-lg overflow-hidden relative">
      <div className="absolute top-4 left-4 z-10 pointer-events-none">
        <h3 className="text-cyber-primary text-xl font-bold neon-text">Network Topology</h3>
        <p className="text-cyber-text text-sm">Live Traffic Flow</p>
      </div>
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        onConnect={onConnect}
        onNodeClick={onNodeClick}
        fitView
        attributionPosition="bottom-right"
      >
        <Background color="#333" gap={20} variant={BackgroundVariant.Dots} />
        <Controls className="!bg-cyber-panel !border-cyber-primary/20" />
      </ReactFlow>
    </div>
  );
};

export default TopologyView;
