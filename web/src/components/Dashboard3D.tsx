import React, { useEffect, useMemo, useRef, useState } from 'react';
import { Canvas, useFrame } from '@react-three/fiber';
import { OrbitControls, Stars } from '@react-three/drei';
import * as THREE from 'three';
import { useNavigate } from 'react-router-dom';
import { getServicesOverview } from '../services/service';

const CyberNode = ({
  position,
  color,
  onClick,
}: {
  position: [number, number, number];
  color: string;
  onClick?: () => void;
}) => {
  const meshRef = useRef<THREE.Mesh>(null);
  const [hovered, setHover] = useState(false);

  useFrame((_, delta) => {
    if (meshRef.current) {
      meshRef.current.rotation.x += delta * 0.5;
      meshRef.current.rotation.y += delta * 0.2;
    }
  });

  return (
    <mesh
      ref={meshRef}
      position={position}
      scale={hovered ? 1.2 : 1}
      onPointerOver={() => setHover(true)}
      onPointerOut={() => setHover(false)}
      onClick={() => onClick?.()}
    >
      <icosahedronGeometry args={[1, 1]} />
      <meshStandardMaterial
        color={color}
        wireframe
        emissive={color}
        emissiveIntensity={0.5}
      />
    </mesh>
  );
};

const Dashboard3D: React.FC = () => {
  const navigate = useNavigate();
  const [services, setServices] = useState<any[]>([]);

  useEffect(() => {
    (async () => {
      try {
        const res: any = await getServicesOverview({ window: '24h' });
        const arr = Array.isArray(res?.services) ? res.services : [];
        setServices(arr.slice(0, 12));
      } catch (e) {
        setServices([]);
      }
    })();
  }, []);

  const nodes = useMemo(() => {
    const r = 2.6;
    return services.map((s, idx) => {
      const angle = (idx / Math.max(1, services.length)) * Math.PI * 2;
      const x = Math.cos(angle) * r;
      const y = Math.sin(angle) * r * 0.6;
      const z = (idx % 2 === 0 ? -1 : 1) * 0.6;
      const status = String(s.status || '');
      const color = status === 'down' ? '#ff4d4f' : status === 'degraded' ? '#faad14' : '#52c41a';
      return {
        key: String(s.service || idx),
        position: [x, y, z] as [number, number, number],
        color,
        service: String(s.service || ''),
      };
    });
  }, [services]);

  return (
    <div className="w-full h-[400px] rounded-lg overflow-hidden border border-cyber-primary/30 bg-cyber-bg relative glass-panel">
      <div className="absolute top-4 left-4 z-10">
        <h3 className="text-cyber-primary text-xl font-bold neon-text">System Core Status</h3>
        <p className="text-cyber-text text-sm">3D Real-time Visualization</p>
      </div>
      <Canvas camera={{ position: [0, 0, 5] }}>
        <ambientLight intensity={0.5} />
        <pointLight position={[10, 10, 10]} />
        <Stars radius={100} depth={50} count={5000} factor={4} saturation={0} fade speed={1} />

        {nodes.map((n) => (
          <CyberNode
            key={n.key}
            position={n.position}
            color={n.color}
            onClick={() => {
              if (!n.service) return;
              navigate(`/services?selected=${encodeURIComponent(n.service)}`);
            }}
          />
        ))}
        
        <OrbitControls enableZoom={false} autoRotate autoRotateSpeed={0.5} />
      </Canvas>
    </div>
  );
};

export default Dashboard3D;
