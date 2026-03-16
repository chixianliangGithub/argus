import React, { useRef } from 'react';
import { ProTable, type ProColumns, type ActionType } from '@ant-design/pro-components';
import { Button, Tag } from 'antd';
import { useLocation, useNavigate } from 'react-router-dom';
import { getIncidents } from '../../services/incident';

const IncidentList: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const navigate = useNavigate();
  const location = useLocation();
  const urlParams = new URLSearchParams(location.search);
  const serviceFilter = urlParams.get('service') || undefined;

  const columns: ProColumns[] = [
    {
      title: '关键词',
      dataIndex: 'q',
      hideInTable: true,
    },
    {
      title: '标题',
      dataIndex: 'title',
      ellipsis: true,
      render: (_: any, record: any) => <a onClick={() => navigate(`/incidents/${record.id}`)}>{record.title || `#${record.id}`}</a>,
    },
    {
      title: '状态',
      dataIndex: 'status',
      valueType: 'select',
      valueEnum: {
        open: { text: 'Open' },
        acked: { text: 'Acked' },
        resolved: { text: 'Resolved' },
      },
      render: (_: any, r: any) => <Tag color={r.status === 'open' ? 'red' : r.status === 'acked' ? 'gold' : 'green'}>{r.status}</Tag>,
      width: 100,
    },
    {
      title: '级别',
      dataIndex: 'severity',
      valueType: 'select',
      valueEnum: {
        critical: { text: 'critical' },
        warning: { text: 'warning' },
        info: { text: 'info' },
      },
      width: 110,
    },
    {
      title: '团队',
      dataIndex: 'team_id',
      width: 90,
    },
    {
      title: '开始时间',
      dataIndex: 'opened_at',
      valueType: 'dateTime',
      hideInSearch: true,
      width: 180,
    },
    {
      title: '最近活动',
      dataIndex: 'last_activity_at',
      valueType: 'dateTime',
      hideInSearch: true,
      width: 180,
    },
  ];

  return (
    <ProTable
      columns={columns}
      actionRef={actionRef}
      cardBordered
      request={async (params) => {
        const res: any = await getIncidents({ ...params, service: serviceFilter });
        const data = Array.isArray(res?.data) ? res.data : res?.data || [];
        const total = res?.total ?? data.length;
        return { data, success: true, total };
      }}
      rowKey="id"
      search={{ labelWidth: 'auto' }}
      pagination={{ pageSize: 20 }}
      dateFormatter="string"
      headerTitle="聚合事件"
      toolBarRender={() => [
        <Tag key="hint" color="default">
          聚合事件 = 多条告警按维度收敛后的处置单
        </Tag>,
        serviceFilter ? (
          <Tag key="svc" color="blue">
            service={serviceFilter}
          </Tag>
        ) : null,
        <Button key="refresh" onClick={() => actionRef.current?.reload()}>
          刷新
        </Button>,
      ]}
    />
  );
};

export default IncidentList;
