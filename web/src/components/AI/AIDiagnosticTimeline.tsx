import React from 'react';
import { Timeline, Card, Tag, Typography, Space } from 'antd';
import { 
  CheckCircleOutlined, 
  ClockCircleOutlined, 
  ExclamationCircleOutlined, 
  LoadingOutlined,
  RobotOutlined
} from '@ant-design/icons';

const { Text } = Typography;

interface DiagnosticEvent {
  timestamp: string;
  status: 'pending' | 'processing' | 'success' | 'warning' | 'error';
  title: string;
  description?: string;
  aiAnalysis?: string;
  confidence?: number;
}

const mockEvents: DiagnosticEvent[] = [
  {
    timestamp: '10:00:05',
    status: 'error',
    title: 'Alert Triggered: High CPU Usage',
    description: 'CPU usage exceeded 90% for 5 minutes on server-01.',
  },
  {
    timestamp: '10:00:08',
    status: 'processing',
    title: 'AI Analysis Started',
    description: 'Argus AI is analyzing logs and metrics...',
    aiAnalysis: 'Initiating deep scan of system logs and recent deployments.',
  },
  {
    timestamp: '10:00:15',
    status: 'warning',
    title: 'Anomaly Detected',
    description: 'Unusual spike in database queries correlated with CPU spike.',
    aiAnalysis: 'Correlation found: 0.95 with DB query spike.',
    confidence: 0.95,
  },
  {
    timestamp: '10:00:30',
    status: 'success',
    title: 'Root Cause Identified',
    description: 'Inefficient query in recent deployment v2.1.0.',
    aiAnalysis: 'The query "SELECT * FROM large_table" is causing full table scan.',
    confidence: 0.98,
  },
  {
    timestamp: '10:00:45',
    status: 'success',
    title: 'Remediation Proposed',
    description: 'Rollback to v2.0.0 or apply index fix.',
    aiAnalysis: 'Recommended Action: Apply index on "large_table.status".',
  }
];

const AIDiagnosticTimeline: React.FC = () => {
  const getIcon = (status: string) => {
    switch (status) {
      case 'processing': return <LoadingOutlined />;
      case 'success': return <CheckCircleOutlined style={{ color: 'green' }} />;
      case 'warning': return <ExclamationCircleOutlined style={{ color: 'orange' }} />;
      case 'error': return <ExclamationCircleOutlined style={{ color: 'red' }} />;
      default: return <ClockCircleOutlined />;
    }
  };

  const getColor = (status: string) => {
    switch (status) {
      case 'processing': return 'blue';
      case 'success': return 'green';
      case 'warning': return 'orange';
      case 'error': return 'red';
      default: return 'gray';
    }
  };

  return (
    <Card 
      title={<Space><RobotOutlined /> AI Diagnostic Timeline</Space>} 
      style={{ width: '100%', maxWidth: 800, margin: '0 auto' }}
      extra={<Tag color="purple">Live Analysis</Tag>}
    >
      <Timeline
        mode="left"
        items={mockEvents.map((event) => ({
          color: getColor(event.status),
          dot: getIcon(event.status),
          children: (
            <div style={{ paddingBottom: 20 }}>
              <Space direction="vertical" size={2} style={{ width: '100%' }}>
                <Space>
                  <Text strong>{event.title}</Text>
                  <Text type="secondary" style={{ fontSize: '12px' }}>{event.timestamp}</Text>
                </Space>
                
                <Text>{event.description}</Text>
                
                {event.aiAnalysis && (
                  <Card 
                    size="small" 
                    style={{ 
                      marginTop: 8, 
                      background: '#f6ffed', 
                      borderColor: '#b7eb8f',
                      borderLeft: '4px solid #52c41a'
                    }}
                  >
                    <Space align="start">
                      <RobotOutlined style={{ color: '#52c41a', marginTop: 4 }} />
                      <div>
                        <Text type="success" strong>AI Insight: </Text>
                        <Text>{event.aiAnalysis}</Text>
                        {event.confidence && (
                          <div>
                            <Tag color="blue" style={{ marginTop: 4 }}>
                              Confidence: {(event.confidence * 100).toFixed(0)}%
                            </Tag>
                          </div>
                        )}
                      </div>
                    </Space>
                  </Card>
                )}
              </Space>
            </div>
          ),
        }))}
      />
    </Card>
  );
};

export default AIDiagnosticTimeline;
