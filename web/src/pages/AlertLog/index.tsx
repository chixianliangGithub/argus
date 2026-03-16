import React, { useMemo, useRef } from 'react';
import { ProTable, type ProColumns, type ActionType } from '@ant-design/pro-components';
import { Tag, Typography } from 'antd';
import { useLocation } from 'react-router-dom';
import { getAlertLogs } from '../../services/alertLog';

const { Text } = Typography;

const useQuery = () => {
  const { search } = useLocation();
  return useMemo(() => new URLSearchParams(search), [search]);
};

const AlertLogList: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const query = useQuery();
  const alertRuleId = query.get('alert_rule_id') || undefined;

  const levelMeta = (level: string) => {
    if (level === 'critical') return { text: '严重', color: '#a8071a' };
    if (level === 'warning') return { text: '警告', color: '#faad14' };
    if (level === 'info') return { text: '信息', color: '#1677ff' };
    return { text: level || '-', color: '#999' };
  };

  const columns: ProColumns[] = [
    {
      title: '时间',
      dataIndex: 'created_at',
      valueType: 'dateTime',
      width: 180,
      hideInSearch: true,
    },
    {
      title: '触发时间范围',
      dataIndex: 'created_at_range',
      valueType: 'dateTimeRange',
      hideInTable: true,
      search: {
        transform: (val: any) => {
          const toISO = (v: any) => {
            if (!v) return v;
            if (typeof v?.toDate === 'function') return v.toDate().toISOString();
            if (v instanceof Date) return v.toISOString();
            if (typeof v === 'string') return v;
            if (typeof v?.toISOString === 'function') return v.toISOString();
            if (typeof v?.format === 'function') return v.format();
            return v;
          };
          const start = Array.isArray(val) ? toISO(val[0]) : undefined;
          const end = Array.isArray(val) ? toISO(val[1]) : undefined;
          return { start_time: start, end_time: end };
        },
      },
    },
    {
      title: '规则ID',
      dataIndex: 'alert_rule_id',
      width: 90,
      initialValue: alertRuleId,
    },
    {
      title: '规则',
      dataIndex: 'rule_name',
      hideInSearch: true,
      width: 220,
      ellipsis: true,
      render: (_, record) => (
        <Text ellipsis={{ tooltip: record.rule_name }} style={{ width: 200 }}>
          {record.rule_name || `#${record.alert_rule_id}`}
        </Text>
      ),
    },
    {
      title: '级别',
      dataIndex: 'level',
      valueType: 'select',
      valueEnum: {
        critical: { text: '严重' },
        warning: { text: '警告' },
        info: { text: '信息' },
      },
      render: (_, record) => {
        const m = levelMeta(record.level);
        const color = record.status === 'resolved' ? '#52c41a' : m.color;
        const text = record.status === 'resolved' ? '恢复' : m.text;
        return <Tag color={color}>{text}</Tag>;
      },
    },
    {
      title: '状态',
      dataIndex: 'status',
      valueType: 'select',
      valueEnum: {
        firing: { text: '触发', status: 'Error' },
        resolved: { text: '恢复', status: 'Success' },
        claimed: { text: '认领', status: 'Processing' },
        unclaimed: { text: '取消认领', status: 'Default' },
        handled: { text: '处理', status: 'Processing' },
        escalation: { text: '升级', status: 'Warning' },
        inhibited: { text: '抑制', status: 'Default' },
        notify_failed: { text: '通知失败', status: 'Error' },
        webhook_debug: { text: 'Webhook调试', status: 'Default' },
      },
      render: (_, record) => (
        <Tag
          color={
            record.status === 'resolved'
              ? '#52c41a'
              : record.status === 'claimed'
                ? 'blue'
                : record.status === 'unclaimed'
                  ? 'default'
                  : record.status === 'handled'
                    ? 'cyan'
                    : record.status === 'escalation'
                      ? 'orange'
                      : record.status === 'inhibited'
                        ? 'default'
                        : record.status === 'webhook_debug'
                          ? 'default'
                      : 'red'
          }
        >
          {record.status === 'firing'
            ? '触发'
            : record.status === 'resolved'
              ? '恢复'
              : record.status === 'claimed'
                ? '认领'
                : record.status === 'unclaimed'
                  ? '取消认领'
                  : record.status === 'handled'
                    ? '处理'
                    : record.status === 'escalation'
                      ? '升级'
            : record.status === 'inhibited'
              ? '抑制'
            : record.status === 'notify_failed'
              ? '通知失败'
            : record.status === 'webhook_debug'
              ? 'Webhook调试'
                      : record.status}
        </Tag>
      ),
    },
    {
      title: '消息',
      dataIndex: 'message',
      ellipsis: true,
      hideInSearch: true,
      render: (_, record) => (
        <Text
          ellipsis={{ tooltip: record.message }}
          style={{
            width: 520,
            color:
              record.status === 'resolved'
                ? '#52c41a'
                : record.level === 'critical'
                  ? '#a8071a'
                  : record.level === 'warning'
                    ? '#faad14'
                    : record.level === 'info'
                      ? '#1677ff'
                      : undefined,
          }}
        >
          {record.message}
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
          const res = await getAlertLogs(params);
          const r: any = res as any;
          const data = Array.isArray(r) ? r : (r.data || []);
          const total = Array.isArray(r) ? data.length : (r.total || data.length);
          return {
            data,
            success: true,
            total,
          };
        } catch (e) {
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
      headerTitle="告警触发记录"
    />
  );
};

export default AlertLogList;
