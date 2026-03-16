import React, { useMemo, useRef, useState } from 'react';
import { ProTable, type ProColumns, type ActionType, ModalForm, ProFormSelect, ProFormTextArea } from '@ant-design/pro-components';
import { Button, Modal, Segmented, Tag, Timeline, Typography, message } from 'antd';
import { batchClaimAlarms, batchHandleAlarms, batchResolveAlarms, batchSilenceAlarms, batchUnclaimAlarms, claimAlarm, getAlarms, handleAlarm, unclaimAlarm } from '../../services/alarm';
import { getAlertLogs } from '../../services/alertLog';
import { useLocation } from 'react-router-dom';

const { Text } = Typography;

const AlarmList: React.FC = () => {
  const location = useLocation();
  const actionRef = useRef<ActionType>();
  const urlParams = useMemo(() => new URLSearchParams(location.search), [location.search]);
  const serviceFilter = urlParams.get('service') || undefined;
  const appFilter = urlParams.get('app') || undefined;
  const statusFilter = urlParams.get('status');
  const [view, setView] = useState<'firing' | 'history'>(statusFilter === 'firing' ? 'firing' : 'history');
  const [eventOpen, setEventOpen] = useState(false);
  const [eventLoading, setEventLoading] = useState(false);
  const [eventRows, setEventRows] = useState<any[]>([]);
  const [eventTitle, setEventTitle] = useState('');
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([]);

  const levelMeta = (level: string) => {
    if (level === 'critical') return { text: '严重', color: '#a8071a' };
    if (level === 'warning') return { text: '警告', color: '#faad14' };
    if (level === 'info') return { text: '信息', color: '#1677ff' };
    return { text: level || '-', color: '#999' };
  };

  const statusText = (s: string) => {
    if (s === 'firing') return '触发';
    if (s === 'resolved') return '恢复';
    if (s === 'claimed') return '认领';
    if (s === 'unclaimed') return '取消认领';
    if (s === 'handled') return '处理';
    if (s === 'escalation') return '升级';
    if (s === 'inhibited') return '抑制';
    return s || '-';
  };

  const statusColor = (s: string) => {
    if (s === 'resolved') return '#52c41a';
    if (s === 'claimed') return 'blue';
    if (s === 'unclaimed') return 'default';
    if (s === 'handled') return 'cyan';
    if (s === 'escalation') return 'orange';
    if (s === 'inhibited') return 'purple';
    if (s === 'firing') return 'red';
    return 'default';
  };

  const openEvents = async (record: any) => {
    setEventOpen(true);
    setEventLoading(true);
    setEventTitle(record.rule_name ? `告警事件 - ${record.rule_name}` : `告警事件 - #${record.id}`);
    try {
      const r: any = await getAlertLogs({ alarm_id: record.id, current: 1, pageSize: 200 });
      const data = Array.isArray(r) ? r : r.data || [];
      setEventRows(data);
    } catch (e) {
      setEventRows([]);
    } finally {
      setEventLoading(false);
    }
  };

  const openEventsByAlarmId = async (alarmId: number, title?: string) => {
    if (!alarmId) return;
    setEventOpen(true);
    setEventLoading(true);
    setEventTitle(title || `告警事件 - #${alarmId}`);
    try {
      const r: any = await getAlertLogs({ alarm_id: alarmId, current: 1, pageSize: 200 });
      const data = Array.isArray(r) ? r : r.data || [];
      setEventRows(data);
    } catch (e) {
      setEventRows([]);
    } finally {
      setEventLoading(false);
    }
  };

  const columns: ProColumns[] = useMemo(
    () => [
      {
        title: 'ID',
        dataIndex: 'id',
        width: 80,
        hideInSearch: true,
        copyable: true,
      },
      {
        title: '时间范围',
        dataIndex: 'start_range',
        valueType: 'dateTimeRange',
        hideInTable: true,
        search: {
          transform: (value: any) => ({
            start_from: value?.[0],
            start_to: value?.[1],
          }),
        },
      },
      {
        title: '认领状态',
        dataIndex: 'claimed_filter',
        valueType: 'select',
        hideInTable: true,
        valueEnum: {
          claimed: { text: '已认领' },
          unclaimed: { text: '未认领' },
        },
        search: {
          transform: (value: any) => ({
            claimed: value === 'claimed' ? '1' : value === 'unclaimed' ? '0' : undefined,
          }),
        },
      },
      {
        title: '认领人',
        dataIndex: 'claimed_by_name',
        hideInTable: true,
      },
      {
        title: '状态',
        dataIndex: 'status',
        valueType: 'select',
        valueEnum: {
          firing: { text: '未恢复' },
          resolved: { text: '已恢复' },
        },
        hideInSearch: true,
        width: 90,
        render: (_, record: any) => (
          <Tag color={record.status === 'firing' ? 'red' : 'green'}>{record.status === 'firing' ? '未恢复' : '已恢复'}</Tag>
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
        hideInSearch: true,
        width: 90,
        render: (_, record: any) => {
          const m = levelMeta(record.level);
          return <Tag color={m.color}>{m.text}</Tag>;
        },
      },
      {
        title: '规则',
        dataIndex: 'rule_name',
        hideInSearch: true,
        width: 220,
        ellipsis: true,
        render: (_, record: any) => (
          <Text ellipsis={{ tooltip: record.rule_name }} style={{ width: 200 }}>
            {record.rule_name || `#${record.alert_rule_id}`}
          </Text>
        ),
      },
      {
        title: '首次发生',
        dataIndex: 'starts_at',
        valueType: 'dateTime',
        width: 180,
        hideInSearch: true,
      },
      {
        title: '最近发生',
        dataIndex: 'last_seen_at',
        valueType: 'dateTime',
        width: 180,
        hideInSearch: true,
        render: (_, record: any) => (record.last_seen_at ? <span>{record.last_seen_at}</span> : <span>-</span>),
      },
      {
        title: '恢复时间',
        dataIndex: 'ends_at',
        valueType: 'dateTime',
        width: 180,
        hideInSearch: true,
        render: (_, record: any) => (record.ends_at ? <span>{record.ends_at}</span> : <span>-</span>),
      },
      {
        title: '认领人',
        dataIndex: 'claimed_by_name',
        width: 140,
        hideInSearch: true,
        render: (_, record: any) => {
          if (!record.claimed_by && !record.claimed_by_name) return <span>-</span>;
          return <span>{record.claimed_by_name || `#${record.claimed_by}`}</span>;
        },
      },
      {
        title: '认领时间',
        dataIndex: 'claimed_at',
        valueType: 'dateTime',
        width: 180,
        hideInSearch: true,
        render: (_, record: any) => (record.claimed_at ? <span>{record.claimed_at}</span> : <span>-</span>),
      },
      {
        title: '处理人',
        dataIndex: 'handled_by_name',
        width: 140,
        hideInSearch: true,
        render: (_, record: any) => {
          if (!record.handled_by && !record.handled_by_name) return <span>-</span>;
          return <span>{record.handled_by_name || `#${record.handled_by}`}</span>;
        },
      },
      {
        title: '处理时间',
        dataIndex: 'handled_at',
        valueType: 'dateTime',
        width: 180,
        hideInSearch: true,
        render: (_, record: any) => (record.handled_at ? <span>{record.handled_at}</span> : <span>-</span>),
      },
      {
        title: '处理结果',
        dataIndex: 'handle_result',
        width: 120,
        hideInSearch: true,
        render: (_, record: any) => (record.handle_result ? <Tag color="cyan">{record.handle_result}</Tag> : <span>-</span>),
      },
      {
        title: '内容',
        dataIndex: 'content',
        hideInSearch: true,
        ellipsis: true,
        render: (_, record: any) => (
          <Text ellipsis={{ tooltip: record.content }} style={{ width: 520 }}>
            {record.content}
          </Text>
        ),
      },
      {
        title: '操作',
        valueType: 'option',
        width: 220,
        render: (_text, record: any) => {
          const claimed = !!record.claimed_by;
          const canHandle = record.status === 'firing';
          const claimLink = (
            claimed ? (
              <a
                onClick={async () => {
                  try {
                    await unclaimAlarm(record.id);
                    message.success('已取消认领');
                    actionRef.current?.reload();
                  } catch (e: any) {
                    const msg = e?.response?.data?.error || e?.message || '取消认领失败';
                    message.error(msg);
                  }
                }}
              >
                取消认领
              </a>
            ) : (
              <a
                onClick={async () => {
                  try {
                    await claimAlarm(record.id);
                    message.success('已认领');
                    actionRef.current?.reload();
                  } catch (e: any) {
                    const msg = e?.response?.data?.error || e?.message || '认领失败';
                    message.error(msg);
                  }
                }}
              >
                认领
              </a>
            )
          );

          const handleLink = canHandle ? (
            <ModalForm
              title="处理告警"
              trigger={<a>处理</a>}
              onFinish={async (values: any) => {
                try {
                  await handleAlarm(record.id, { result: values.result, note: values.note });
                  message.success('已记录处理信息');
                  actionRef.current?.reload();
                  return true;
                } catch (e) {
                  message.error('处理失败');
                  return false;
                }
              }}
              initialValues={{
                result: record.handle_result || '已处理',
                note: record.handle_note || '',
              }}
            >
              <ProFormSelect
                name="result"
                label="处理结果"
                rules={[{ required: true }]}
                options={[
                  { label: '已处理', value: '已处理' },
                  { label: '已确认', value: '已确认' },
                  { label: '无需处理', value: '无需处理' },
                  { label: '已转交', value: '已转交' },
                ]}
              />
              <ProFormTextArea name="note" label="备注" fieldProps={{ rows: 4 }} />
            </ModalForm>
          ) : null;

          const eventLink = (
            <a
              onClick={() => {
                openEvents(record);
              }}
            >
              事件
            </a>
          );

          return [claimLink, handleLink, eventLink].filter(Boolean) as any;
        },
      },
    ],
    []
  );

  return (
    <>
      <ProTable
        columns={columns}
        actionRef={actionRef}
        cardBordered
        request={async (params) => {
          try {
            const r: any = await getAlarms({
              ...params,
              status: view === 'firing' ? 'firing' : undefined,
              service: serviceFilter,
              app: appFilter,
            });
            const data = Array.isArray(r) ? r : r.data || [];
            const total = Array.isArray(r) ? data.length : r.total || data.length;
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
        rowSelection={{
          selectedRowKeys,
          onChange: (keys) => setSelectedRowKeys(keys),
        }}
        search={{
          labelWidth: 'auto',
        }}
        pagination={{
          pageSize: 20,
        }}
        dateFormatter="string"
        headerTitle="告警列表"
        toolBarRender={() => [
          serviceFilter ? (
            <Tag key="svc" color="blue">
              service={serviceFilter}
            </Tag>
          ) : null,
          appFilter ? (
            <Tag key="app" color="default">
              app={appFilter}
            </Tag>
          ) : null,
          <Segmented
            key="seg"
            value={view}
            onChange={(v) => {
              setView(v as any);
              actionRef.current?.reload();
            }}
            options={[
              { label: '未恢复', value: 'firing' },
              { label: '历史', value: 'history' },
            ]}
          />,
          <Button
            key="batchClaim"
            disabled={selectedRowKeys.length === 0}
            onClick={async () => {
              try {
                const ids = selectedRowKeys.map((x) => Number(x)).filter((x) => Number.isFinite(x));
                const r: any = await batchClaimAlarms(ids);
                const ok = r?.succeeded_ids?.length ?? 0;
                const fail = r?.failed_ids?.length ?? 0;
                if (fail > 0) {
                  message.warning(`批量认领完成：成功${ok}，失败${fail}`);
                } else {
                  message.success(`批量认领完成：成功${ok}`);
                }
                setSelectedRowKeys([]);
                actionRef.current?.reload();
              } catch (e: any) {
                const msg = e?.response?.data?.error || e?.message || '批量认领失败';
                message.error(msg);
              }
            }}
          >
            批量认领
          </Button>,
          <Button
            key="batchUnclaim"
            disabled={selectedRowKeys.length === 0}
            onClick={async () => {
              try {
                const ids = selectedRowKeys.map((x) => Number(x)).filter((x) => Number.isFinite(x));
                const r: any = await batchUnclaimAlarms(ids);
                const ok = r?.succeeded_ids?.length ?? 0;
                const fail = r?.failed_ids?.length ?? 0;
                if (fail > 0) {
                  message.warning(`批量取消认领完成：成功${ok}，失败${fail}`);
                } else {
                  message.success(`批量取消认领完成：成功${ok}`);
                }
                setSelectedRowKeys([]);
                actionRef.current?.reload();
              } catch (e: any) {
                const msg = e?.response?.data?.error || e?.message || '批量取消认领失败';
                message.error(msg);
              }
            }}
          >
            批量取消认领
          </Button>,
          <ModalForm
            key="batchHandle"
            title="批量处理告警"
            trigger={<Button disabled={selectedRowKeys.length === 0 || view !== 'firing'}>批量处理</Button>}
            onFinish={async (values: any) => {
              try {
                const ids = selectedRowKeys.map((x) => Number(x)).filter((x) => Number.isFinite(x));
                const r: any = await batchHandleAlarms(ids, { result: values.result, note: values.note });
                const ok = r?.succeeded_ids?.length ?? 0;
                const fail = r?.failed_ids?.length ?? 0;
                if (fail > 0) {
                  message.warning(`批量处理完成：成功${ok}，失败${fail}`);
                } else {
                  message.success(`批量处理完成：成功${ok}`);
                }
                setSelectedRowKeys([]);
                actionRef.current?.reload();
                return true;
              } catch (e: any) {
                const msg = e?.response?.data?.error || e?.message || '批量处理失败';
                message.error(msg);
                return false;
              }
            }}
            initialValues={{
              result: '已处理',
              note: '',
            }}
          >
            <ProFormSelect
              name="result"
              label="处理结果"
              rules={[{ required: true }]}
              options={[
                { label: '已处理', value: '已处理' },
                { label: '已确认', value: '已确认' },
                { label: '无需处理', value: '无需处理' },
                { label: '已转交', value: '已转交' },
              ]}
            />
            <ProFormTextArea name="note" label="备注" fieldProps={{ rows: 4 }} />
          </ModalForm>,
          <ModalForm
            key="batchSilence"
            title="批量静默告警"
            trigger={<Button disabled={selectedRowKeys.length === 0 || view !== 'firing'}>批量静默</Button>}
            onFinish={async (values: any) => {
              try {
                const ids = selectedRowKeys.map((x) => Number(x)).filter((x) => Number.isFinite(x));
                const mins = Number(values.duration_mins);
                await batchSilenceAlarms(ids, { duration_mins: mins, comment: values.comment });
                message.success('已创建静默规则');
                setSelectedRowKeys([]);
                actionRef.current?.reload();
                return true;
              } catch (e: any) {
                const msg = e?.response?.data?.error || e?.message || '批量静默失败';
                message.error(msg);
                return false;
              }
            }}
            initialValues={{
              duration_mins: 60,
              comment: '',
            }}
          >
            <ProFormSelect
              name="duration_mins"
              label="静默时长"
              rules={[{ required: true }]}
              options={[
                { label: '15 分钟', value: 15 },
                { label: '30 分钟', value: 30 },
                { label: '1 小时', value: 60 },
                { label: '4 小时', value: 240 },
                { label: '24 小时', value: 1440 },
              ]}
              width="sm"
            />
            <ProFormTextArea name="comment" label="备注(可选)" fieldProps={{ rows: 3 }} />
          </ModalForm>,
          <Button
            key="batchResolve"
            danger
            disabled={selectedRowKeys.length === 0 || view !== 'firing'}
            onClick={async () => {
              const ok = await new Promise<boolean>((resolve) => {
                Modal.confirm({
                  title: '确认批量关闭告警？',
                  content: '将把选中的告警标记为已恢复（手动关闭）。',
                  onOk: () => resolve(true),
                  onCancel: () => resolve(false),
                });
              });
              if (!ok) return;
              try {
                const ids = selectedRowKeys.map((x) => Number(x)).filter((x) => Number.isFinite(x));
                const r: any = await batchResolveAlarms(ids);
                const okn = r?.succeeded_ids?.length ?? 0;
                const fail = r?.failed_ids?.length ?? 0;
                if (fail > 0) {
                  message.warning(`批量关闭完成：成功${okn}，失败${fail}`);
                } else {
                  message.success(`批量关闭完成：成功${okn}`);
                }
                setSelectedRowKeys([]);
                actionRef.current?.reload();
              } catch (e: any) {
                const msg = e?.response?.data?.error || e?.message || '批量关闭失败';
                message.error(msg);
              }
            }}
          >
            批量关闭
          </Button>,
          <Button key="reload" onClick={() => actionRef.current?.reload()}>
            刷新
          </Button>,
        ]}
      />

      <Modal title={eventTitle} open={eventOpen} onCancel={() => setEventOpen(false)} footer={null} width={900}>
        {eventLoading ? (
          <div>加载中...</div>
        ) : (
          <Timeline
            items={[...eventRows].reverse().map((x: any) => ({
              color: statusColor(x.status),
              children: (
                <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                  <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                    <Tag color={statusColor(x.status)}>{statusText(x.status)}</Tag>
                    <span style={{ color: '#999' }}>{x.created_at || x.triggered_at || ''}</span>
                    {x.status === 'inhibited' && typeof x.message === 'string' && x.message.includes('source_alarm_id=') ? (
                      <a
                        onClick={() => {
                          const m = x.message.match(/source_alarm_id=(\d+)/);
                          const id = m ? Number(m[1]) : 0;
                          if (id) openEventsByAlarmId(id, `源告警事件 - #${id}`);
                        }}
                      >
                        查看源告警
                      </a>
                    ) : null}
                  </div>
                  <pre style={{ margin: 0, whiteSpace: 'pre-wrap' }}>{x.message}</pre>
                </div>
              ),
            }))}
          />
        )}
      </Modal>
    </>
  );
};

export default AlarmList;
