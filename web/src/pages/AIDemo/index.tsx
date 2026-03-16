import React, { useEffect, useMemo, useRef, useState } from 'react';
import { Card, Button, Divider, Input, Select, message, Checkbox, Collapse, Drawer, Table, Tag } from 'antd';
import { RobotOutlined } from '@ant-design/icons';
import { aiInsight } from '../../services/ai';
import { getDataNames } from '../../services/dataname';
import { getIncidents, getIncident } from '../../services/incident';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { queryData } from '../../services/query';
import EChartsView from '../../components/EChartsView';

const AIDemo: React.FC = () => {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const incidentId = Number(searchParams.get('incident_id') || 0);
  const serviceParam = String(searchParams.get('service') || '').trim();

  const [dsLoading, setDsLoading] = useState(false);
  const [dataNames, setDataNames] = useState<any[]>([]);
  const [dataNameId, setDataNameId] = useState<number | undefined>(undefined);

  const [incidentLoading, setIncidentLoading] = useState(false);
  const [incidents, setIncidents] = useState<any[]>([]);
  const [selectedIncidentId, setSelectedIncidentId] = useState<number | undefined>(incidentId || undefined);
  const [incidentSearch, setIncidentSearch] = useState('');
  const incidentSearchTimer = useRef<number | undefined>(undefined);
  const [incidentDetail, setIncidentDetail] = useState<any>(null);
  const [contextParts, setContextParts] = useState<string[]>(['summary', 'alarms', 'logs', 'rca']);
  const [question, setQuestion] = useState('请基于上下文给出诊断建议，并给出验证步骤与处置建议。');

  const [aiMsg, setAiMsg] = useState('');
  const [aiLoading, setAiLoading] = useState(false);
  const [aiResult, setAiResult] = useState<any>(null);
  const [aiMeta, setAiMeta] = useState<{ startedAt?: number; ms?: number } | null>(null);
  const [previewOpen, setPreviewOpen] = useState(false);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [previewRows, setPreviewRows] = useState<any[]>([]);

  const selectedIncident = useMemo(() => incidents.find((x) => x.id === selectedIncidentId), [incidents, selectedIncidentId]);
  const selectedDN = useMemo(() => dataNames.find((x) => x.id === dataNameId), [dataNames, dataNameId]);
  const selectedDS = useMemo(() => selectedDN?.data_source, [selectedDN]);

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

  const loadIncidents = async (q?: string) => {
    setIncidentLoading(true);
    try {
      const res = await getIncidents({ current: 1, pageSize: 20, q: q || undefined });
      const arr = Array.isArray(res?.data) ? res.data : res?.data || [];
      setIncidents(arr);
    } catch (e) {
      setIncidents([]);
    } finally {
      setIncidentLoading(false);
    }
  };

  const loadIncidentDetail = async (id: number) => {
    if (!id) return;
    try {
      const res: any = await getIncident(id);
      setIncidentDetail(res);
    } catch (e) {}
  };

  useEffect(() => {
    loadDatasources();
    loadIncidents();
  }, []);

  useEffect(() => {
    if (selectedIncidentId) {
      loadIncidentDetail(selectedIncidentId);
    } else {
      setIncidentDetail(null);
    }
  }, [selectedIncidentId]);

  useEffect(() => {
    if (incidentSearchTimer.current) {
      window.clearTimeout(incidentSearchTimer.current);
    }
    incidentSearchTimer.current = window.setTimeout(() => {
      loadIncidents(incidentSearch.trim());
    }, 300);
    return () => {
      if (incidentSearchTimer.current) window.clearTimeout(incidentSearchTimer.current);
    };
  }, [incidentSearch]);

  const buildContext = () => {
    if (serviceParam && !incidentDetail?.incident) {
      return `[Service]\nservice=${serviceParam}`;
    }
    if (!incidentDetail?.incident) return '';
    const inc = incidentDetail.incident;
    const alarms = Array.isArray(incidentDetail?.alarms) ? incidentDetail.alarms : [];
    const logs = Array.isArray(incidentDetail?.logs) ? incidentDetail.logs : [];
    const rca = Array.isArray(incidentDetail?.rca) ? incidentDetail.rca : [];

    const parts: string[] = [];
    parts.push(`IncidentID: ${inc.id}`);
    if (inc.title) parts.push(`Title: ${inc.title}`);
    if (inc.status) parts.push(`Status: ${inc.status}`);
    if (inc.severity) parts.push(`Severity: ${inc.severity}`);
    if (inc.team_id) parts.push(`TeamID: ${inc.team_id}`);
    if (inc.opened_at) parts.push(`OpenedAt: ${inc.opened_at}`);
    if (inc.last_activity_at) parts.push(`LastActivityAt: ${inc.last_activity_at}`);

    const blocks: string[] = [];
    blocks.push(`[Incident Meta]\n${parts.join('\n')}`);

    if (contextParts.includes('summary')) {
      blocks.push(`[Incident Summary]\n${inc.summary || '-'}`);
    }

    if (contextParts.includes('alarms')) {
      const top = alarms.slice(0, 20).map((a: any) => {
        const ss = [a.id ? `id=${a.id}` : '', a.rule_name ? `rule=${a.rule_name}` : '', a.status ? `status=${a.status}` : '', a.starts_at ? `starts_at=${a.starts_at}` : '', a.fingerprint ? `fp=${a.fingerprint}` : '']
          .filter(Boolean)
          .join(' ');
        return `- ${ss}`;
      });
      blocks.push(`[Related Alarms] (top ${top.length})\n${top.join('\n') || '-'}`);
    }

    if (contextParts.includes('logs')) {
      const top = logs.slice(0, 50).map((l: any) => {
        const t = l.created_at || l.triggered_at || '';
        const s = l.status || '';
        const msg = String(l.message || '').slice(0, 240);
        return `- ${t} ${s} ${msg}`;
      });
      blocks.push(`[Timeline Logs] (latest ${top.length})\n${top.join('\n') || '-'}`);
    }

    if (contextParts.includes('rca')) {
      const top = rca.slice(0, 10).map((r: any) => {
        const t = r.created_at || '';
        const a = r.alarm_id ? `alarm_id=${r.alarm_id}` : '';
        const root = String(r.root_cause || '').slice(0, 240);
        return `- ${t} ${a} ${root}`.trim();
      });
      blocks.push(`[RCA Reports] (latest ${top.length})\n${top.join('\n') || '-'}`);
    }

    return blocks.join('\n\n');
  };

  const buildMessage = () => {
    const q = question.trim();
    const ctx = buildContext();
    if (!ctx) return q;
    return `Question:\n${q}\n\nContext:\n${ctx}`;
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      <Card title={<span><RobotOutlined /> AI 诊断中心</span>}>
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          <Select
            style={{ width: 360 }}
            loading={incidentLoading}
            placeholder="选择事故（可选）"
            allowClear
            value={selectedIncidentId}
            showSearch
            filterOption={false}
            onSearch={(v) => setIncidentSearch(v)}
            onChange={(v) => setSelectedIncidentId(v ? Number(v) : undefined)}
            options={incidents.map((i) => ({ label: `${i.title || `#${i.id}`}`, value: i.id }))}
          />
          <Button
            disabled={!selectedIncidentId}
            onClick={() => {
              if (selectedIncidentId) navigate(`/incidents/${selectedIncidentId}`);
            }}
          >
            打开事故
          </Button>
          <Select
            style={{ width: 360 }}
            loading={dsLoading}
            value={dataNameId}
            onChange={(v) => setDataNameId(Number(v))}
            options={dataNames.map((d) => ({ label: `${d.name} · ${d?.data_source?.name} (${d?.data_source?.type})`, value: d.id }))}
          />
          {selectedDS ? <Tag color="blue">{selectedDS.type}</Tag> : null}
          <Button
            type="primary"
            loading={aiLoading}
            disabled={!dataNameId || (!aiMsg.trim() && !question.trim())}
            onClick={async () => {
              if (!dataNameId) return;
              const dn = dataNames.find((x) => x.id === dataNameId);
              const dsId = Number(dn?.data_source?.id || dn?.data_source_id || 0);
              if (!dsId) {
                message.error('该数据名未绑定数据源');
                return;
              }
              const msg = aiMsg.trim() ? aiMsg : buildMessage();
              if (!msg.trim()) {
                message.error('请输入问题或生成输入');
                return;
              }
              setAiLoading(true);
              setAiMeta({ startedAt: Date.now() });
              try {
                if (!aiMsg.trim()) {
                  setAiMsg(msg);
                }
                const res = await aiInsight({ datasource_id: dsId, message: msg });
                setAiResult(res);
                setAiMeta((prev) => ({ ...prev, ms: prev?.startedAt ? Date.now() - prev.startedAt : undefined }));
                message.success('生成完成');
              } catch (e) {
                setAiResult(null);
                setAiMeta(null);
                message.error('生成失败');
              } finally {
                setAiLoading(false);
              }
            }}
          >
            生成洞察
          </Button>
        </div>

        {selectedIncident ? (
          <div style={{ marginTop: 8, color: '#999' }}>当前事故：{selectedIncident.title || `#${selectedIncident.id}`}</div>
        ) : null}
        {!selectedIncident && serviceParam ? <div style={{ marginTop: 8, color: '#999' }}>当前服务：{serviceParam}</div> : null}

        <Divider />
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          <Select
            style={{ width: 360 }}
            placeholder="快捷模板"
            allowClear
            options={[
              { label: '根因定位', value: '请基于事故上下文定位最可能根因，并给出证据与验证步骤。' },
              { label: '影响面分析', value: '请基于事故上下文分析影响面（服务/集群/用户/时间窗）并给出优先级。' },
              { label: '处置建议', value: '请基于事故上下文给出处置建议（应急、止血、根治、回滚、验证）。' },
              { label: '复盘提纲', value: '请生成复盘提纲（时间线、根因、影响、处置、改进项、Owner、截止时间）。' },
            ]}
            onChange={(v) => {
              if (typeof v === 'string') setQuestion(v);
            }}
          />
          <Input.TextArea value={question} onChange={(e) => setQuestion(e.target.value)} rows={3} placeholder="你的问题" />
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', alignItems: 'center' }}>
            <Checkbox.Group
              value={contextParts}
              onChange={(v) => setContextParts(v as string[])}
              options={[
                { label: '摘要', value: 'summary' },
                { label: '关联告警', value: 'alarms' },
                { label: '时间线', value: 'logs' },
                { label: 'RCA', value: 'rca' },
              ]}
            />
            <Button
              disabled={!question.trim()}
              onClick={() => {
                setAiMsg(buildMessage());
              }}
            >
              填充上下文
            </Button>
            <Button
              disabled={!question.trim()}
              onClick={() => {
                const msg = buildMessage();
                setAiMsg(msg);
              }}
            >
              重建输入
            </Button>
          </div>
          <Collapse
            size="small"
            items={[
              {
                key: 'preview',
                label: '上下文预览',
                children: <pre style={{ margin: 0, whiteSpace: 'pre-wrap' }}>{buildMessage() || '-'}</pre>,
              },
            ]}
          />
          <Input.TextArea value={aiMsg} onChange={(e) => setAiMsg(e.target.value)} rows={10} placeholder="最终输入（可编辑）" />
        </div>
      </Card>

      <Card title="洞察结果" size="small">
        {aiResult ? (
          <div>
            <div style={{ marginBottom: 8 }}>
              <b>Summary</b>：{aiResult.summary || '-'}
            </div>
            <div style={{ marginBottom: 8 }}>
              <b>Query</b>：<pre style={{ margin: 0, whiteSpace: 'pre-wrap' }}>{aiResult.query || '-'}</pre>
            </div>
            <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginBottom: 8 }}>
              <Button
                disabled={!aiResult?.query}
                onClick={async () => {
                  try {
                    await navigator.clipboard.writeText(String(aiResult.query || ''));
                    message.success('已复制查询');
                  } catch (e) {
                    message.error('复制失败');
                  }
                }}
              >
                复制查询
              </Button>
              <Button
                disabled={!dataNameId || !aiResult?.query}
                loading={previewLoading}
                onClick={async () => {
                  if (!dataNameId) return;
                  const dn = dataNames.find((x) => x.id === dataNameId);
                  const dsId = Number(dn?.data_source?.id || dn?.data_source_id || 0);
                  if (!dsId) return;
                  setPreviewOpen(true);
                  setPreviewLoading(true);
                  try {
                    const r: any = await queryData({ datasource_id: dsId, query: String(aiResult.query || '') });
                    const data = Array.isArray(r?.data) ? r.data : r?.data || [];
                    setPreviewRows(Array.isArray(data) ? data : []);
                  } catch (e) {
                    setPreviewRows([]);
                  } finally {
                    setPreviewLoading(false);
                  }
                }}
              >
                预览数据
              </Button>
              {aiMeta?.ms ? <Tag color="default">耗时 {aiMeta.ms}ms</Tag> : null}
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
        ) : (
          <div style={{ color: '#999' }}>暂无数据</div>
        )}
      </Card>

      <Drawer title="查询结果预览" open={previewOpen} onClose={() => setPreviewOpen(false)} width={900}>
        <Table
          rowKey={(_, idx) => String(idx)}
          size="small"
          loading={previewLoading}
          dataSource={previewRows}
          pagination={{ pageSize: 20 }}
          scroll={{ x: true }}
          columns={
            previewRows.length > 0 && typeof previewRows[0] === 'object' && previewRows[0] !== null
              ? Object.keys(previewRows[0]).map((k) => ({ title: k, dataIndex: k, key: k }))
              : []
          }
        />
        {previewRows.length === 0 && !previewLoading ? <div style={{ color: '#999' }}>暂无数据</div> : null}
      </Drawer>
    </div>
  );
};

export default AIDemo;
