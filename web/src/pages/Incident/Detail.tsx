import React, { useEffect, useMemo, useState } from 'react';
import { Card, Descriptions, Button, Table, Tag, message, Divider, Input, Select, Modal, Timeline, Collapse } from 'antd';
import { useNavigate, useParams } from 'react-router-dom';
import { ackIncident, commentIncident, exportIncidentMarkdown, getIncident, resolveIncident, updateIncident } from '../../services/incident';
import { getDataNames } from '../../services/dataname';
import { createIncidentAIInsight, getIncidentAIInsights } from '../../services/incidentAiInsight';
import { matchRunbooks, executeRunbook } from '../../services/runbook';
import EChartsView from '../../components/EChartsView';

const IncidentDetail: React.FC = () => {
  const { id } = useParams();
  const incidentId = Number(id);
  const navigate = useNavigate();

  const [loading, setLoading] = useState(false);
  const [data, setData] = useState<any>(null);

  const [dsLoading, setDsLoading] = useState(false);
  const [dataNames, setDataNames] = useState<any[]>([]);
  const [dataNameId, setDataNameId] = useState<number | undefined>(undefined);
  const [insightsLoading, setInsightsLoading] = useState(false);
  const [insights, setInsights] = useState<any[]>([]);
  const [aiMsg, setAiMsg] = useState('');
  const [aiLoading, setAiLoading] = useState(false);
  const [aiResult, setAiResult] = useState<any>(null);

  const [rbLoading, setRbLoading] = useState(false);
  const [rbList, setRbList] = useState<any[]>([]);

  const [ackOpen, setAckOpen] = useState(false);
  const [ackNote, setAckNote] = useState('');
  const [resolveOpen, setResolveOpen] = useState(false);
  const [resolveNote, setResolveNote] = useState('');
  const [commentOpen, setCommentOpen] = useState(false);
  const [commentNote, setCommentNote] = useState('');

  const [editOpen, setEditOpen] = useState(false);
  const [editSaving, setEditSaving] = useState(false);
  const [editForm, setEditForm] = useState<any>({
    title: '',
    summary: '',
    impact: '',
    root_cause: '',
    classification: '',
    mitigation: '',
    verification: '',
    rollback_plan: '',
    follow_ups: '',
    tags: '',
  });

  const incident = data?.incident;
  const alarms = Array.isArray(data?.alarms) ? data.alarms : [];
  const logs = Array.isArray(data?.logs) ? data.logs : [];
  const rca = Array.isArray(data?.rca) ? data.rca : [];
  const activity = Array.isArray(data?.activity) ? data.activity : [];

  const title = useMemo(() => {
    if (!incident) return `事件 #${incidentId}`;
    return incident.title || `事件 #${incidentId}`;
  }, [incident, incidentId]);

  const openEdit = () => {
    if (!incident) return;
    setEditForm({
      title: incident.title || '',
      summary: incident.summary || '',
      impact: incident.impact || '',
      root_cause: incident.root_cause || '',
      classification: incident.classification || '',
      mitigation: incident.mitigation || '',
      verification: incident.verification || '',
      rollback_plan: incident.rollback_plan || '',
      follow_ups: incident.follow_ups || '',
      tags: incident.tags || '',
    });
    setEditOpen(true);
  };

  const load = async () => {
    if (!incidentId) return;
    setLoading(true);
    try {
      const res: any = await getIncident(incidentId);
      setData(res);
      const s = res?.incident?.summary || '';
      setAiMsg((prev) => (prev ? prev : `请基于以下事故信息给出诊断建议：\n\n${s}`));
      const rid = res?.recommended_dataname_id;
      if (!dataNameId && rid) {
        setDataNameId(Number(rid));
      }
    } catch (e) {
      setData(null);
    } finally {
      setLoading(false);
    }
  };

  const loadDatasources = async () => {
    setDsLoading(true);
    try {
      const res = await getDataNames();
      const arr = Array.isArray(res) ? res : res?.data || [];
      setDataNames(arr);
      if (!dataNameId && arr.length > 0) {
        setDataNameId(arr[0].id);
      }
    } catch (e) {
      setDataNames([]);
    } finally {
      setDsLoading(false);
    }
  };

  useEffect(() => {
    load();
    loadDatasources();
    (async () => {
      if (!incidentId) return;
      setInsightsLoading(true);
      try {
        const res: any = await getIncidentAIInsights(incidentId, { current: 1, pageSize: 20 });
        const arr = Array.isArray(res?.data) ? res.data : [];
        setInsights(arr);
      } catch (e) {
        setInsights([]);
      } finally {
        setInsightsLoading(false);
      }
    })();
  }, [incidentId]);

  const statusTag = (s: string) => {
    if (s === 'open') return <Tag color="red">open</Tag>;
    if (s === 'acked') return <Tag color="gold">acked</Tag>;
    if (s === 'resolved') return <Tag color="#52c41a">resolved</Tag>;
    return <Tag>{s}</Tag>;
  };

  const timelineItems = useMemo(() => {
    const items: Array<{ t: string; node: React.ReactNode }> = [];
    for (const a of activity) {
      const t = a.created_at || '';
      const typ = String(a.type || '');
      const msg = String(a.message || '');
      const color =
        typ === 'resolved' ? '#52c41a' : typ === 'acked' ? '#faad14' : typ === 'ai_insight' ? '#1677ff' : typ === 'runbook' ? '#13c2c2' : '#1677ff';
      const suffix = typ === 'ai_insight' && a.ref_type === 'incident_ai_insight' && a.ref_id ? `#${a.ref_id}` : '';
      const label =
        typ === 'resolved'
          ? '关闭'
          : typ === 'acked'
            ? 'ACK'
            : typ === 'ai_insight'
              ? `AI${suffix}`
              : typ === 'runbook'
                ? 'Runbook'
                : '备注';
      items.push({
        t,
        node: (
          <div style={{ display: 'flex', gap: 8, alignItems: 'flex-start' }}>
            <Tag color={color}>{label}</Tag>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
              <div style={{ whiteSpace: 'pre-wrap' }}>{msg || '-'}</div>
              {typ === 'runbook' && a.ref_type === 'runbook_execution' && a.ref_id ? (
                <div>
                  <a onClick={() => navigate(`/executions?execution_id=${a.ref_id}`)}>查看执行详情</a>
                </div>
              ) : null}
            </div>
          </div>
        ),
      });
    }
    for (const l of logs) {
      const t = l.created_at || l.triggered_at || '';
      const st = String(l.status || '');
      const color =
        st === 'resolved'
          ? '#52c41a'
          : st === 'claimed'
            ? '#1677ff'
            : st === 'unclaimed'
              ? 'default'
              : st === 'handled'
                ? 'cyan'
                : st === 'escalation'
                  ? 'orange'
                  : 'red';
      const label =
        st === 'firing'
          ? '触发'
          : st === 'resolved'
            ? '恢复'
            : st === 'claimed'
              ? '认领'
              : st === 'unclaimed'
                ? '取消认领'
                : st === 'handled'
                  ? '处理'
                  : st === 'escalation'
                    ? '升级'
                    : st;
      items.push({
        t,
        node: (
          <div style={{ display: 'flex', gap: 8, alignItems: 'flex-start' }}>
            <Tag color={color}>{label}</Tag>
            <div style={{ whiteSpace: 'pre-wrap' }}>{String(l.message || '').slice(0, 800) || '-'}</div>
          </div>
        ),
      });
    }
    items.sort((a, b) => String(b.t).localeCompare(String(a.t)));
    return items.map((x) => ({ children: x.node }));
  }, [activity, logs]);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      <Card
        title={title}
        loading={loading}
        extra={
          <div style={{ display: 'flex', gap: 8 }}>
            <Button onClick={() => navigate('/incidents')}>返回列表</Button>
            <Button disabled={!incidentId} onClick={openEdit}>
              编辑复盘
            </Button>
            <Button
              disabled={!incidentId}
              onClick={async () => {
                try {
                  const blob: any = await exportIncidentMarkdown(incidentId);
                  const url = window.URL.createObjectURL(blob);
                  const a = document.createElement('a');
                  a.href = url;
                  a.download = `incident-${incidentId}.md`;
                  a.click();
                  window.URL.revokeObjectURL(url);
                } catch (e) {
                  message.error('导出失败');
                }
              }}
            >
              导出Markdown
            </Button>
            <Button
              disabled={!incidentId || incident?.status === 'resolved'}
              onClick={async () => {
                setAckOpen(true);
              }}
            >
              ACK
            </Button>
            <Button
              disabled={!incidentId}
              onClick={() => setCommentOpen(true)}
            >
              添加备注
            </Button>
            <Button
              danger
              disabled={!incidentId || incident?.status === 'resolved'}
              onClick={async () => {
                setResolveOpen(true);
              }}
            >
              关闭
            </Button>
          </div>
        }
      >
        <Descriptions column={2} size="small">
          <Descriptions.Item label="状态">{statusTag(incident?.status)}</Descriptions.Item>
          <Descriptions.Item label="级别">{incident?.severity || '-'}</Descriptions.Item>
          <Descriptions.Item label="团队">{incident?.team_id || '-'}</Descriptions.Item>
          <Descriptions.Item label="聚合Key">{incident?.dedup_key || '-'}</Descriptions.Item>
          <Descriptions.Item label="开始时间">{incident?.opened_at || '-'}</Descriptions.Item>
          <Descriptions.Item label="最近活动">{incident?.last_activity_at || '-'}</Descriptions.Item>
        </Descriptions>
        <Divider />
        <div style={{ whiteSpace: 'pre-wrap' }}>{incident?.summary || '-'}</div>
        <Divider />
        <Collapse
          items={[
            {
              key: 'post',
              label: '复盘字段',
              children: (
                <Descriptions column={1} size="small">
                  <Descriptions.Item label="影响面">{incident?.impact || '-'}</Descriptions.Item>
                  <Descriptions.Item label="根因">{incident?.root_cause || '-'}</Descriptions.Item>
                  <Descriptions.Item label="根因分类">{incident?.classification || '-'}</Descriptions.Item>
                  <Descriptions.Item label="处置步骤">{incident?.mitigation || '-'}</Descriptions.Item>
                  <Descriptions.Item label="验证">{incident?.verification || '-'}</Descriptions.Item>
                  <Descriptions.Item label="回滚">{incident?.rollback_plan || '-'}</Descriptions.Item>
                  <Descriptions.Item label="Follow-ups">{incident?.follow_ups || '-'}</Descriptions.Item>
                  <Descriptions.Item label="标签">{incident?.tags || '-'}</Descriptions.Item>
                </Descriptions>
              ),
            },
          ]}
        />
      </Card>

      <Card title="关联告警" size="small" loading={loading}>
        <Table
          rowKey="id"
          size="small"
          pagination={false}
          dataSource={alarms}
          columns={[
            { title: 'AlarmID', dataIndex: 'id', width: 90 },
            { title: '状态', dataIndex: 'status', width: 100 },
            { title: '规则', dataIndex: 'rule_name', width: 240, ellipsis: true },
            { title: '级别', dataIndex: 'level', width: 100 },
            {
              title: '基线',
              dataIndex: 'payload',
              width: 260,
              render: (_: any, r: any) => {
                const raw = String(r?.payload || '').trim();
                if (!raw) return <span>-</span>;
                try {
                  const obj = JSON.parse(raw);
                  const b = obj?.baseline;
                  if (!b) return <span>-</span>;
                  const method = String(b.method || '');
                  const k = b.k != null ? Number(b.k) : undefined;
                  const lower = b.lower != null ? Number(b.lower) : undefined;
                  const upper = b.upper != null ? Number(b.upper) : undefined;
                  if (!method && lower == null && upper == null) return <span>-</span>;
                  const range =
                    lower != null && upper != null ? (
                      <span style={{ fontFamily: 'monospace' }}>
                        [{lower.toFixed(2)}, {upper.toFixed(2)}]
                      </span>
                    ) : (
                      <span>-</span>
                    );
                  return (
                    <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
                      {method ? <Tag color="blue">{method}</Tag> : null}
                      {k != null ? <Tag color="default">k={k.toFixed(1)}</Tag> : null}
                      {range}
                    </div>
                  );
                } catch (e) {
                  return <span>-</span>;
                }
              },
            },
            { title: '触发时间', dataIndex: 'starts_at', width: 190 },
            { title: '指纹', dataIndex: 'fingerprint', ellipsis: true },
          ]}
        />
      </Card>

      <Card title="处置时间线" size="small" loading={loading}>
        <Timeline mode="left" items={timelineItems} />
      </Card>

      <Card title="RCA 报告" size="small" loading={loading}>
        <Table
          rowKey="id"
          size="small"
          pagination={false}
          dataSource={rca}
          columns={[
            { title: 'ID', dataIndex: 'id', width: 80 },
            { title: 'AlarmID', dataIndex: 'alarm_id', width: 90 },
            { title: '创建时间', dataIndex: 'created_at', width: 190 },
            { title: '根因', dataIndex: 'root_cause', ellipsis: true },
          ]}
        />
      </Card>

      <Card title="AI 洞察" size="small">
        <div style={{ display: 'flex', gap: 8, marginBottom: 8 }}>
          <Select
            style={{ width: 320 }}
            loading={dsLoading}
            value={dataNameId}
            onChange={(v) => setDataNameId(Number(v))}
            options={(() => {
              const cands = Array.isArray((data as any)?.dataname_candidates) ? (data as any).dataname_candidates : [];
              if (cands.length > 0) {
                const recommend = cands.map((d: any) => ({
                  label: `${d.name} · ${d.data_source_name} (${d.data_source_type}) · ${d.alarm_count}`,
                  value: d.id,
                }));
                const all = dataNames.map((d) => ({ label: `${d.name} · ${d?.data_source?.name} (${d?.data_source?.type})`, value: d.id }));
                return [
                  { label: '推荐', options: recommend },
                  { label: '全部数据名', options: all },
                ];
              }
              return dataNames.map((d) => ({ label: `${d.name} · ${d?.data_source?.name} (${d?.data_source?.type})`, value: d.id }));
            })() as any}
          />
          <Button
            type="primary"
            loading={aiLoading}
            disabled={!dataNameId || !aiMsg}
            onClick={async () => {
              if (!dataNameId) return;
              const dn = dataNames.find((x) => x.id === dataNameId);
              const dsId = Number(dn?.data_source?.id || dn?.data_source_id || 0);
              if (!dsId) {
                setAiResult(null);
                return;
              }
              setAiLoading(true);
              try {
                const res = await createIncidentAIInsight(incidentId, { datasource_id: dsId, message: aiMsg });
                setAiResult(res);
                const listRes: any = await getIncidentAIInsights(incidentId, { current: 1, pageSize: 20 });
                const arr = Array.isArray(listRes?.data) ? listRes.data : [];
                setInsights(arr);
              } catch (e) {
                setAiResult(null);
              } finally {
                setAiLoading(false);
              }
            }}
          >
            生成并保存
          </Button>
        </div>
        <div style={{ marginBottom: 8, color: 'rgba(0,0,0,0.65)' }}>
          洞察将从所选数据源拉取相关数据生成结论；选错数据源可能导致洞察偏差。
        </div>
        <Input.TextArea value={aiMsg} onChange={(e) => setAiMsg(e.target.value)} rows={6} />
        {aiResult ? (
          <div style={{ marginTop: 12 }}>
            <Divider />
            <div style={{ marginBottom: 8 }}>
              <b>Summary</b>：{aiResult.summary || '-'}
            </div>
            <div style={{ marginBottom: 8 }}>
              <b>Query</b>：<pre style={{ margin: 0, whiteSpace: 'pre-wrap' }}>{aiResult.query || '-'}</pre>
            </div>
            <div>
              <EChartsView option={aiResult.chart_config} height={420} />
              <Collapse
                size="small"
                style={{ marginTop: 8 }}
                items={[
                  {
                    key: 'chart',
                    label: 'ChartConfig（高级）',
                    children: <pre style={{ margin: 0, whiteSpace: 'pre-wrap' }}>{JSON.stringify(aiResult.chart_config || {}, null, 2)}</pre>,
                  },
                ]}
              />
            </div>
          </div>
        ) : null}
        <Divider />
        <Collapse
          size="small"
          items={[
            {
              key: 'history',
              label: `历史洞察（${insights.length}）`,
              children: (
                <div>
                  {insightsLoading ? (
                    <div>加载中...</div>
                  ) : insights.length === 0 ? (
                    <div>-</div>
                  ) : (
                    insights.map((x: any) => (
                      <Card key={x.id} size="small" style={{ marginBottom: 8 }}>
                        <div style={{ display: 'flex', justifyContent: 'space-between', gap: 8, marginBottom: 6 }}>
                          <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
                            <Tag color="blue">datasource_id={x.datasource_id}</Tag>
                            <Tag>{x.created_at || '-'}</Tag>
                          </div>
                          <Button
                            size="small"
                            onClick={() => {
                              setAiMsg(x.message || aiMsg);
                              const dn = dataNames.find((d) => Number(d?.data_source?.id || d?.data_source_id || 0) === Number(x.datasource_id));
                              if (dn?.id) setDataNameId(Number(dn.id));
                              setAiResult(x);
                            }}
                          >
                            查看
                          </Button>
                        </div>
                        <div style={{ whiteSpace: 'pre-wrap' }}>{x.summary || '-'}</div>
                      </Card>
                    ))
                  )}
                </div>
              ),
            },
          ]}
        />
      </Card>

      <Card title="Runbook 推荐与执行" size="small">
        <div style={{ display: 'flex', gap: 8, marginBottom: 8 }}>
          <Button
            type="primary"
            loading={rbLoading}
            disabled={!incident?.summary}
            onClick={async () => {
              setRbLoading(true);
              try {
                const res = await matchRunbooks({ query: incident?.summary || '' });
                const arr = Array.isArray((res as any)?.matches) ? (res as any).matches : Array.isArray((res as any)?.data) ? (res as any).data : (res as any)?.data || [];
                setRbList(Array.isArray(arr) ? arr : []);
              } catch (e) {
                setRbList([]);
              } finally {
                setRbLoading(false);
              }
            }}
          >
            推荐 Runbook
          </Button>
        </div>
        <Table
          rowKey="id"
          size="small"
          pagination={false}
          dataSource={rbList}
          columns={[
            { title: 'ID', dataIndex: 'id', width: 80 },
            { title: '名称', dataIndex: 'name', width: 240, ellipsis: true },
            { title: '描述', dataIndex: 'description', ellipsis: true },
            {
              title: '操作',
              width: 120,
              render: (_: any, record: any) => (
                <Button
                  onClick={async () => {
                    try {
                      const res: any = await executeRunbook(record.id, { context: { incident_id: incidentId } });
                      if (res?.id) {
                        message.success('已发起执行');
                        navigate('/executions');
                      } else {
                        message.success('已发起执行');
                      }
                    } catch (e) {
                      message.error('执行失败');
                    }
                  }}
                >
                  执行
                </Button>
              ),
            },
          ]}
        />
      </Card>

      <Modal
        title="ACK"
        open={ackOpen}
        okText="确认"
        cancelText="取消"
        okButtonProps={{ disabled: !ackNote.trim() }}
        onCancel={() => {
          setAckOpen(false);
          setAckNote('');
        }}
        onOk={async () => {
          try {
            await ackIncident(incidentId, ackNote);
            message.success('已 ACK');
            setAckOpen(false);
            setAckNote('');
            load();
          } catch (e) {
            message.error('ACK 失败');
          }
        }}
      >
        <Input.TextArea value={ackNote} onChange={(e) => setAckNote(e.target.value)} rows={5} placeholder="处置说明（必填）" />
      </Modal>

      <Modal
        title="编辑复盘字段"
        open={editOpen}
        confirmLoading={editSaving}
        onCancel={() => setEditOpen(false)}
        onOk={async () => {
          if (!incidentId) return;
          setEditSaving(true);
          try {
            await updateIncident(incidentId, editForm);
            message.success('已保存');
            setEditOpen(false);
            await load();
          } catch (e) {
            message.error('保存失败');
          } finally {
            setEditSaving(false);
          }
        }}
        width={900}
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          <Input placeholder="标题" value={editForm.title} onChange={(e) => setEditForm((p: any) => ({ ...p, title: e.target.value }))} />
          <Input.TextArea
            rows={3}
            placeholder="摘要/现象"
            value={editForm.summary}
            onChange={(e) => setEditForm((p: any) => ({ ...p, summary: e.target.value }))}
          />
          <Input.TextArea
            rows={3}
            placeholder="影响面"
            value={editForm.impact}
            onChange={(e) => setEditForm((p: any) => ({ ...p, impact: e.target.value }))}
          />
          <Input.TextArea
            rows={3}
            placeholder="根因"
            value={editForm.root_cause}
            onChange={(e) => setEditForm((p: any) => ({ ...p, root_cause: e.target.value }))}
          />
          <Input
            placeholder="根因分类"
            value={editForm.classification}
            onChange={(e) => setEditForm((p: any) => ({ ...p, classification: e.target.value }))}
          />
          <Input.TextArea
            rows={3}
            placeholder="处置步骤"
            value={editForm.mitigation}
            onChange={(e) => setEditForm((p: any) => ({ ...p, mitigation: e.target.value }))}
          />
          <Input.TextArea
            rows={3}
            placeholder="验证"
            value={editForm.verification}
            onChange={(e) => setEditForm((p: any) => ({ ...p, verification: e.target.value }))}
          />
          <Input.TextArea
            rows={3}
            placeholder="回滚"
            value={editForm.rollback_plan}
            onChange={(e) => setEditForm((p: any) => ({ ...p, rollback_plan: e.target.value }))}
          />
          <Input.TextArea
            rows={3}
            placeholder="Follow-ups"
            value={editForm.follow_ups}
            onChange={(e) => setEditForm((p: any) => ({ ...p, follow_ups: e.target.value }))}
          />
          <Input placeholder="标签(可选)" value={editForm.tags} onChange={(e) => setEditForm((p: any) => ({ ...p, tags: e.target.value }))} />
        </div>
      </Modal>

      <Modal
        title="关闭"
        open={resolveOpen}
        okText="确认"
        cancelText="取消"
        okButtonProps={{ disabled: !resolveNote.trim() }}
        onCancel={() => {
          setResolveOpen(false);
          setResolveNote('');
        }}
        onOk={async () => {
          try {
            await resolveIncident(incidentId, resolveNote);
            message.success('已关闭');
            setResolveOpen(false);
            setResolveNote('');
            load();
          } catch (e) {
            message.error('关闭失败');
          }
        }}
      >
        <Input.TextArea value={resolveNote} onChange={(e) => setResolveNote(e.target.value)} rows={5} placeholder="关闭原因/结论（必填）" />
      </Modal>

      <Modal
        title="添加备注"
        open={commentOpen}
        okText="提交"
        cancelText="取消"
        okButtonProps={{ disabled: !commentNote.trim() }}
        onCancel={() => {
          setCommentOpen(false);
          setCommentNote('');
        }}
        onOk={async () => {
          try {
            await commentIncident(incidentId, commentNote);
            message.success('已添加备注');
            setCommentOpen(false);
            setCommentNote('');
            load();
          } catch (e) {
            message.error('添加失败');
          }
        }}
      >
        <Input.TextArea value={commentNote} onChange={(e) => setCommentNote(e.target.value)} rows={5} placeholder="备注内容（必填）" />
      </Modal>
    </div>
  );
};

export default IncidentDetail;
