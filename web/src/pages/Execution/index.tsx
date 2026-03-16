import React, { useEffect, useRef, useState } from 'react';
import { ProTable, type ProColumns, type ActionType } from '@ant-design/pro-components';
import { Tag, Button, Modal, Timeline, Typography, Card, Input, message } from 'antd';
import { getExecutions, getExecution, approveExecution } from '../../services/execution';
import { CheckCircleOutlined, ClockCircleOutlined, CloseCircleOutlined, SyncOutlined } from '@ant-design/icons';
import { useLocation } from 'react-router-dom';

const { Text } = Typography;
const { TextArea } = Input;

const ExecutionList: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [detailVisible, setDetailVisible] = useState(false);
  const [currentExecution, setCurrentExecution] = useState<any>(null);
  const [feedback, setFeedback] = useState('');
  const location = useLocation();
  const lastAutoOpenIdRef = useRef<number>(0);

  const handleDetail = async (record: any) => {
    try {
      const res = await getExecution(record.id);
      setCurrentExecution(res);
      setDetailVisible(true);
    } catch (error) {
      message.error('获取详情失败');
    }
  };

  useEffect(() => {
    const sp = new URLSearchParams(location.search || '');
    const id = Number(sp.get('execution_id') || 0);
    if (!id || !Number.isFinite(id)) return;
    if (lastAutoOpenIdRef.current === id) return;
    lastAutoOpenIdRef.current = id;
    handleDetail({ id });
  }, [location.search]);

  const handleApprove = async (approved: boolean) => {
    if (!currentExecution) return;
    try {
      await approveExecution(currentExecution.id, approved, feedback);
      message.success(approved ? '已批准' : '已拒绝');
      setDetailVisible(false);
      actionRef.current?.reload();
    } catch (error) {
      message.error('操作失败');
    }
  };

  const columns: ProColumns[] = [
    {
      title: 'ID',
      dataIndex: 'id',
      width: 60,
      hideInSearch: true,
    },
    {
      title: 'Runbook',
      dataIndex: ['runbook', 'name'],
      ellipsis: true,
    },
    {
      title: '状态',
      dataIndex: 'status',
      valueType: 'select',
      valueEnum: {
        pending: { text: '等待中', status: 'Default' },
        analyzing: { text: 'AI 分析中', status: 'Processing' },
        waiting_approval: { text: '待审批', status: 'Warning' },
        executing: { text: '执行中', status: 'Processing' },
        completed: { text: '已完成', status: 'Success' },
        failed: { text: '失败', status: 'Error' },
        cancelled: { text: '已取消', status: 'Default' },
      },
      render: (_, record) => {
        let color = 'default';
        let icon = <ClockCircleOutlined />;
        switch (record.status) {
          case 'analyzing':
          case 'executing':
            color = 'processing';
            icon = <SyncOutlined spin />;
            break;
          case 'waiting_approval':
            color = 'warning';
            icon = <ClockCircleOutlined />;
            break;
          case 'completed':
            color = 'success';
            icon = <CheckCircleOutlined />;
            break;
          case 'failed':
            color = 'error';
            icon = <CloseCircleOutlined />;
            break;
        }
        return (
          <Tag icon={icon} color={color}>
            {record.status}
          </Tag>
        );
      },
    },
    {
      title: '触发时间',
      dataIndex: 'created_at',
      valueType: 'dateTime',
      sorter: true,
    },
    {
      title: '操作',
      valueType: 'option',
      render: (_, record) => [
        <a key="detail" onClick={() => handleDetail(record)}>
          详情
        </a>,
      ],
    },
  ];

  return (
    <>
      <ProTable
        columns={columns}
        actionRef={actionRef}
        cardBordered
        request={async (params) => {
          try {
            const res = await getExecutions(params);
            const data = Array.isArray(res) ? res : (res.data || []);
            return {
              data: data,
              success: true,
              total: data.length,
            };
          } catch (error) {
            return {
              data: [],
              success: false,
            };
          }
        }}
        rowKey="id"
        search={{
          labelWidth: 'auto',
        }}
        pagination={{
          pageSize: 10,
        }}
        dateFormatter="string"
        headerTitle="执行记录列表"
        polling={5000} // Auto refresh every 5s
      />

      <Modal
        title={`执行详情 #${currentExecution?.id}`}
        open={detailVisible}
        onCancel={() => setDetailVisible(false)}
        footer={null}
        width={800}
      >
        {currentExecution && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
            <Card title="基本信息" size="small">
              <p><strong>Runbook:</strong> {currentExecution.runbook?.name}</p>
              <p><strong>状态:</strong> {currentExecution.status}</p>
              <p><strong>开始时间:</strong> {currentExecution.created_at}</p>
            </Card>

            <Card title="执行日志" size="small">
              <Timeline mode="left">
                {(() => {
                  try {
                    const logs = JSON.parse(currentExecution.logs || '[]');
                    return logs.map((log: string, index: number) => (
                      <Timeline.Item key={index}>{log}</Timeline.Item>
                    ));
                  } catch (e) {
                    return <Timeline.Item>无法解析日志</Timeline.Item>;
                  }
                })()}
              </Timeline>
            </Card>

            {currentExecution.status === 'waiting_approval' && (
              <Card title="审批操作" size="small" style={{ borderColor: '#faad14' }}>
                <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
                  <Text type="warning">此任务需要您的审批才能继续执行。</Text>
                  <TextArea 
                    rows={3} 
                    placeholder="请输入审批意见或反馈..." 
                    value={feedback}
                    onChange={(e) => setFeedback(e.target.value)}
                  />
                  <div style={{ display: 'flex', gap: 12, justifyContent: 'flex-end' }}>
                    <Button danger onClick={() => handleApprove(false)}>拒绝</Button>
                    <Button type="primary" onClick={() => handleApprove(true)}>批准执行</Button>
                  </div>
                </div>
              </Card>
            )}
          </div>
        )}
      </Modal>
    </>
  );
};

export default ExecutionList;
