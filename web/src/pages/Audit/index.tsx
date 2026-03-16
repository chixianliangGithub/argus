import React, { useRef } from 'react';
import { ProTable, type ProColumns, type ActionType } from '@ant-design/pro-components';
import { Tag, Typography } from 'antd';
import { getAuditLogs } from '../../services/audit';

const { Text } = Typography;

const AuditLogList: React.FC = () => {
  const actionRef = useRef<ActionType>();

  const columns: ProColumns[] = [
    {
      title: '时间',
      dataIndex: 'created_at',
      valueType: 'dateTime',
      sorter: true,
      width: 180,
    },
    {
      title: '用户',
      dataIndex: 'username',
      copyable: true,
    },
    {
      title: '动作',
      dataIndex: 'action',
      render: (_, record) => {
        let color = 'default';
        if (record.action === 'POST') color = 'green';
        if (record.action === 'PUT') color = 'orange';
        if (record.action === 'DELETE') color = 'red';
        return <Tag color={color}>{record.action}</Tag>;
      },
      filters: true,
      onFilter: true,
    },
    {
      title: '资源',
      dataIndex: 'resource',
      ellipsis: true,
    },
    {
      title: 'IP地址',
      dataIndex: 'client_ip',
      hideInSearch: true,
    },
    {
      title: '详情',
      dataIndex: 'details',
      hideInSearch: true,
      ellipsis: true,
      render: (_, record) => (
        <Text ellipsis={{ tooltip: record.details }} style={{ width: 300 }}>
          {record.details}
        </Text>
      ),
    },
  ];

  return (
    <ProTable
      columns={columns}
      actionRef={actionRef}
      cardBordered
      request={async (params) => {
        try {
          const res = await getAuditLogs(params);
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
        pageSize: 20,
      }}
      dateFormatter="string"
      headerTitle="操作审计日志"
    />
  );
};

export default AuditLogList;
