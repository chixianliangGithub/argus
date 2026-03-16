import React, { useEffect, useMemo, useRef, useState } from 'react';
import { ProTable, type ProColumns, type ActionType, ProFormGroup, ProFormList } from '@ant-design/pro-components';
import { Button, message, Popconfirm, Tabs, Card, Modal, Table, Dropdown, Tooltip } from 'antd';
import { PlusOutlined, PlayCircleOutlined } from '@ant-design/icons';
import { getAlertRules, getAlertRule, addAlertRule, updateAlertRule, deleteAlertRule, previewAlertRule, triggerAlertRule, batchUpdateAlertRulesEnabled, getAlertRuleRuntime } from '../../services/alertRule';
import { getAlertEvalRecords } from '../../services/alertEvalRecord';
import { getDataNames } from '../../services/dataname';
import { getTemplates, addTemplate } from '../../services/template';
import { getTeams } from '../../services/team';
import { getUsers } from '../../services/user';
import { getNotificationChannels } from '../../services/notificationChannel';
import { getEscalations } from '../../services/escalation';
import { getSilences } from '../../services/silence';
import { ModalForm, ProFormText, ProFormSelect, ProFormTextArea, ProFormDigit, ProFormDependency, ProFormSwitch, ProFormDatePicker, type ProFormInstance } from '@ant-design/pro-components';
import { useLocation, useNavigate } from 'react-router-dom';

const AlertRule: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const navigate = useNavigate();
  const location = useLocation();
  const createFormRef = useRef<ProFormInstance>();
  const copyFormRef = useRef<ProFormInstance>();
  const editFormRef = useRef<ProFormInstance>();
  const [dataNames, setDataNames] = useState<any[]>([]);
  const [templates, setTemplates] = useState<any[]>([]);
  const [teams, setTeams] = useState<any[]>([]);
  const [users, setUsers] = useState<any[]>([]);
  const [notificationChannels, setNotificationChannels] = useState<any[]>([]);
  const [escalations, setEscalations] = useState<any[]>([]);
  const [silences, setSilences] = useState<any[]>([]);
  const [createOpen, setCreateOpen] = useState(false);
  const [createInitialValues, setCreateInitialValues] = useState<any>({});
  const [activeFormTab, setActiveFormTab] = useState<string>('basic');
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([]);
  const [runtimeOpen, setRuntimeOpen] = useState(false);
  const [runtimeLoading, setRuntimeLoading] = useState(false);
  const [runtimeData, setRuntimeData] = useState<any>(null);
  const [evalLoading, setEvalLoading] = useState(false);
  const [evalData, setEvalData] = useState<any[]>([]);
  
  const [previewData, setPreviewData] = useState<any[]>([]);
  const [previewColumns, setPreviewColumns] = useState<string[]>([]);
  const [previewModalVisible, setPreviewModalVisible] = useState(false);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [currentFormValues, setCurrentFormValues] = useState<any>({});

  const buildTemplateSuggestion = (cols: string[]) => {
    const big = new Set(['log_message', 'message', 'msg', 'stack', 'stacktrace', 'exception', 'content', 'body']);
    const lines: string[] = [];
    lines.push('规则：{{ .Rule.Name }}');
    lines.push('级别：{{ .Rule.Level }}');
    lines.push('触发时间：{{ .TriggerAt }}');
    lines.push('发生时间：{{ .EventAt }}');
    lines.push('发送时间：{{ .SendAt }}');
    lines.push('命中数：{{ .Count }}');
    lines.push('值：{{ .Value }}');
    if (cols.includes('trace_id')) {
      lines.push(`trace_id：{{ default "-" (index .Row "trace_id") }}`);
    }
    if (cols.includes('service_name')) {
      lines.push(`service_name：{{ default "-" (index .Row "service_name") }}`);
    }
    if (cols.includes('level')) {
      lines.push(`level：{{ default "-" (index .Row "level") }}`);
    }
    if (cols.includes('happen_time')) {
      lines.push(`happen_time：{{ default "-" (index .Row "happen_time") }}`);
    }
    lines.push('样例字段：');
    const showCols = cols.slice(0, 20);
    for (const k of showCols) {
      if (!k) continue;
      if (big.has(k)) {
        lines.push(`${k}：{{ truncate (default "" (index .Row "${k}")) 200 }}`);
      } else {
        lines.push(`${k}：{{ default "-" (index .Row "${k}") }}`);
      }
    }
    lines.push('内容：');
    lines.push('{{ .Content }}');
    return lines.join('\n');
  };

  useEffect(() => {
    const st: any = (location as any).state;
    if (st?.prefill) {
      setCreateInitialValues(st.prefill);
      setCreateOpen(true);
      setCurrentFormValues(st.prefill);
      loadData();
      setTimeout(() => {
        createFormRef.current?.setFieldsValue?.(st.prefill);
      }, 0);
    }
  }, [location]);

  const buildCronExpression = (values: any) => {
    const mode = values?.schedule_mode || 'every';
    if (mode === 'advanced') {
      return values?.cron_expression || '';
    }

    const seconds = 0;
    const parseTime = (t: string) => {
      if (!t || typeof t !== 'string') return null;
      const parts = t.split(':');
      if (parts.length !== 2) return null;
      const h = Number(parts[0]);
      const m = Number(parts[1]);
      if (Number.isNaN(h) || Number.isNaN(m)) return null;
      return { h, m };
    };

    if (mode === 'every') {
      const n = Number(values?.every_minutes || 5);
      return `${seconds} */${n} * * * *`;
    }
    if (mode === 'hourly') {
      const m = Number(values?.hourly_minute || 0);
      return `${seconds} ${m} * * * *`;
    }
    if (mode === 'daily') {
      const tm = parseTime(values?.daily_time);
      if (!tm) return '';
      return `${seconds} ${tm.m} ${tm.h} * * *`;
    }
    if (mode === 'weekly') {
      const tm = parseTime(values?.weekly_time);
      const days: number[] = values?.weekly_days || [];
      if (!tm || !Array.isArray(days) || days.length === 0) return '';
      const dow = days
        .map((d) => (d === 7 ? 0 : d))
        .sort((a, b) => a - b)
        .join(',');
      return `${seconds} ${tm.m} ${tm.h} * * ${dow}`;
    }
    if (mode === 'monthly') {
      const tm = parseTime(values?.monthly_time);
      const days: number[] = values?.monthly_days || [];
      if (!tm || !Array.isArray(days) || days.length === 0) return '';
      const dom = days.sort((a, b) => a - b).join(',');
      return `${seconds} ${tm.m} ${tm.h} ${dom} * *`;
    }
    return '';
  };

  const loadData = async () => {
    try {
      const [dnRes, tplRes, teamRes, userRes, escRes, silRes] = await Promise.all([
        getDataNames(), 
        getTemplates(), 
        getTeams(),
        getUsers(),
        getEscalations(),
        getSilences(),
      ]);
      setDataNames(Array.isArray(dnRes) ? dnRes : (dnRes.data || []));
      setTemplates(Array.isArray(tplRes) ? tplRes : (tplRes.data || []));
      setTeams(Array.isArray(teamRes) ? teamRes : (teamRes.data || []));
      setUsers(Array.isArray(userRes) ? userRes : (userRes.data || []));
      setEscalations(Array.isArray(escRes) ? escRes : (escRes.data || []));
      setSilences(Array.isArray(silRes) ? silRes : (silRes.data || []));
      const chRes = await getNotificationChannels();
      setNotificationChannels(Array.isArray(chRes) ? chRes : (chRes.data || []));
    } catch (error) {
      console.error(error);
    }
  };

  const handlePreview = async (vals?: any) => {
    const dataNameId = vals?.data_name_id ?? currentFormValues?.data_name_id;
    const query = vals?.query ?? currentFormValues?.query;
    const evalWindowMinutes = vals?.eval_window_minutes ?? currentFormValues?.eval_window_minutes;
    if (!dataNameId || !query) {
      message.warning('请先选择数据名并输入查询语句');
      return;
    }
    setPreviewLoading(true);
    try {
      const res = await previewAlertRule({ data_name_id: dataNameId, query, eval_window_minutes: evalWindowMinutes });
      const data = res.data || [];
      // Normalize data to array of objects for Table
      let tableData = [];
      if (Array.isArray(data)) {
        tableData = data;
      } else if (typeof data === 'object') {
        // Handle single object or map
        tableData = [data];
      }
      setPreviewData(tableData);
      if (tableData.length > 0 && typeof tableData[0] === 'object' && tableData[0] !== null) {
        setPreviewColumns(Object.keys(tableData[0]));
      } else {
        setPreviewColumns([]);
      }
      setPreviewModalVisible(true);
      message.success('查询成功');
    } catch (error) {
      console.error(error);
      message.error('查询失败');
    } finally {
      setPreviewLoading(false);
    }
  };

  const openRuntime = async (ruleId: number) => {
    setRuntimeOpen(true);
    setRuntimeLoading(true);
    setEvalLoading(true);
    try {
      const res = await getAlertRuleRuntime(ruleId);
      setRuntimeData(res);
    } catch (e) {
      setRuntimeData(null);
    } finally {
      setRuntimeLoading(false);
    }
    try {
      const res2 = await getAlertEvalRecords({ alert_rule_id: ruleId, current: 1, pageSize: 20 });
      const data2 = res2?.data || [];
      setEvalData(Array.isArray(data2) ? data2 : []);
    } catch (e) {
      setEvalData([]);
    } finally {
      setEvalLoading(false);
    }
  };

  const renderFormItems = () => (
    <Tabs destroyInactiveTabPane={false} activeKey={activeFormTab} onChange={setActiveFormTab} items={[
      {
        key: 'basic',
        label: '基本信息',
        forceRender: true,
        children: (
          <>
            <ProFormGroup title="基础信息">
              <ProFormText name="name" label="规则名称" rules={[{ required: true }]} width="md" />
              <ProFormSelect
                name="team_id"
                label="所属团队"
                options={teams.map((t) => ({ label: t.name, value: t.id }))}
                rules={[{ required: true }]}
                width="md"
              />
              <ProFormSwitch name="is_enabled" label="启用状态" initialValue={true} />
            </ProFormGroup>
            <ProFormTextArea name="description" label="描述" width="xl" />
          </>
        )
      },
      {
        key: 'data',
        label: '数据配置',
        forceRender: true,
        children: (
          <>
            <ProFormSelect
              name="data_name_id"
              label="数据名"
              options={dataNames.map((dn) => ({ label: dn.name, value: dn.id }))}
              rules={[{ required: true }]}
              width="md"
            />
            <ProFormTextArea 
              name="query" 
              label="查询语句" 
              rules={[{ required: true }]} 
              fieldProps={{ rows: 5, style: { fontFamily: 'monospace' } }}
              placeholder="SELECT count(*) FROM errors WHERE status = 500"
            />
            <ProFormDependency name={['data_name_id']}>
              {({ data_name_id }) => {
                const dn = dataNames.find((x) => x.id === data_name_id);
                const dsType = dn?.data_source?.type;
                if (dsType === 'prometheus') return null;
                return (
                  <>
                    <ProFormGroup>
                      <ProFormSelect
                        name="value_mode"
                        label="触发值来源"
                        initialValue="row_count"
                        options={[
                          { label: '返回行数', value: 'row_count' },
                          { label: '字段数值', value: 'field' },
                          { label: '自动', value: 'auto' },
                        ]}
                        width="sm"
                      />
                      <ProFormSelect
                        name="row_pick"
                        label="取样行"
                        initialValue="first"
                        tooltip="用于消息模板取值：从返回结果中取第一条或最后一条作为样例/输出"
                        options={[
                          { label: '第一条', value: 'first' },
                          { label: '最后一条', value: 'last' },
                        ]}
                        width="sm"
                      />
                    </ProFormGroup>
                    <ProFormDependency name={['value_mode']}>
                      {({ value_mode }) => {
                        if (value_mode === 'row_count') return null;
                        return (
                          <ProFormSelect
                            name="value_field"
                            label="值字段"
                            placeholder="用于提取数值参与阈值/算法计算"
                            options={previewColumns.map((c) => ({ label: c, value: c }))}
                            fieldProps={{ allowClear: true, showSearch: true }}
                            width="md"
                          />
                        );
                      }}
                    </ProFormDependency>
                  </>
                );
              }}
            </ProFormDependency>
            <ProFormDependency name={['data_name_id', 'query']}>
              {({ data_name_id, query }) => (
                <Button
                  type="primary"
                  icon={<PlayCircleOutlined />}
                  onClick={() => handlePreview({ data_name_id, query })}
                  loading={previewLoading}
                  style={{ marginBottom: 16 }}
                >
                  预览数据
                </Button>
              )}
            </ProFormDependency>
          </>
        )
      },
      {
        key: 'alert',
        label: '告警逻辑',
        forceRender: true,
        children: (
          <>
            <ProFormSelect
              name="level"
              label="告警级别"
              options={[
                { label: '严重', value: 'critical' },
                { label: '警告', value: 'warning' },
                { label: '信息', value: 'info' },
              ]}
              rules={[{ required: true }]}
              width="sm"
            />
            <ProFormSelect
              name="algorithm"
              label="检测算法"
              options={[
                { label: '静态阈值 (Static)', value: 'static' },
                { label: '智能基线 (Baseline)', value: 'baseline_auto' },
                { label: '3-Sigma 异常检测', value: '3sigma' },
                { label: 'MAD 离群检测', value: 'mad' },
                { label: '环比 (MoM)', value: 'mom' },
                { label: '同比 (YoY)', value: 'yoy' },
              ]}
              initialValue="static"
              rules={[{ required: true }]}
              width="md"
            />
            
            <ProFormDependency name={['algorithm']}>
              {({ algorithm }) => {
                if (algorithm === 'static') {
                  return (
                    <ProFormGroup>
                      <ProFormSelect
                        name="condition"
                        label="判定条件"
                        options={[
                          { label: '>', value: '>' },
                          { label: '<', value: '<' },
                          { label: '=', value: '=' },
                          { label: '>=', value: '>=' },
                          { label: '<=', value: '<=' },
                        ]}
                        rules={[{ required: true }]}
                        initialValue=">"
                        width="sm"
                      />
                      <ProFormDigit name="threshold" label="阈值" rules={[{ required: true }]} width="md" />
                    </ProFormGroup>
                  );
                }
                if (algorithm === 'baseline_auto') {
                  return (
                    <>
                      <ProFormGroup>
                        <ProFormDigit
                          name="baseline_sensitivity_k"
                          label="敏感度 (k)"
                          tooltip="偏离基线超过 kσ 触发，常用 3"
                          initialValue={3}
                          min={1}
                          width="sm"
                        />
                        <ProFormSelect
                          name="baseline_seasonality"
                          label="周期"
                          initialValue="none"
                          options={[
                            { label: '无', value: 'none' },
                            { label: '日周期', value: 'daily' },
                            { label: '周周期', value: 'weekly' },
                          ]}
                          width="sm"
                        />
                      </ProFormGroup>
                      <ProFormDependency name={['baseline_sensitivity_k', 'baseline_seasonality']}>
                        {({ baseline_sensitivity_k, baseline_seasonality }) => (
                          <Card title="参数预览" size="small">
                            <div style={{ fontFamily: 'monospace' }}>
                              {JSON.stringify({
                                sensitivity_k: Number(baseline_sensitivity_k || 3),
                                seasonality: baseline_seasonality || 'none',
                                method: 'auto',
                              })}
                            </div>
                          </Card>
                        )}
                      </ProFormDependency>
                    </>
                  );
                }
                if (algorithm === 'mom' || algorithm === 'yoy') {
                  return (
                    <>
                      <ProFormGroup>
                        <ProFormText name="compare_window" label="窗口" initialValue="1h" width="sm" />
                        <ProFormText name="compare_offset" label="对比偏移" initialValue={algorithm === 'yoy' ? '365d' : '1h'} width="sm" />
                        <ProFormDigit name="compare_change_percent" label="变化阈值(%)" initialValue={20} width="sm" />
                        <ProFormSelect
                          name="compare_direction"
                          label="方向"
                          initialValue="both"
                          options={[
                            { label: '上升', value: 'increase' },
                            { label: '下降', value: 'decrease' },
                            { label: '双向', value: 'both' },
                          ]}
                          width="sm"
                        />
                      </ProFormGroup>
                    </>
                  );
                }
                if (algorithm === '3sigma') {
                  return (
                    <>
                      <ProFormGroup>
                        <ProFormText
                          name="algo_window"
                          label="窗口"
                          tooltip="用于计算均值/方差的历史窗口，示例：30m、1h、6h"
                          initialValue="1h"
                          width="sm"
                        />
                        <ProFormDigit
                          name="algo_n"
                          label="N"
                          tooltip="阈值倍数，常用 3"
                          initialValue={3}
                          min={1}
                          width="sm"
                        />
                      </ProFormGroup>
                      <ProFormDependency name={['algo_window', 'algo_n']}>
                        {({ algo_window, algo_n }) => (
                          <Card title="参数预览" size="small">
                            <div style={{ fontFamily: 'monospace' }}>
                              {JSON.stringify({ window: algo_window || '1h', n: Number(algo_n || 3) })}
                            </div>
                          </Card>
                        )}
                      </ProFormDependency>
                    </>
                  );
                }
                if (algorithm === 'mad') {
                  return (
                    <>
                      <ProFormGroup>
                        <ProFormText
                          name="algo_window"
                          label="窗口"
                          tooltip="用于计算中位数/MAD 的历史窗口，示例：30m、1h、6h"
                          initialValue="1h"
                          width="sm"
                        />
                        <ProFormDigit
                          name="algo_k"
                          label="K"
                          tooltip="阈值倍数，常用 3"
                          initialValue={3}
                          min={1}
                          width="sm"
                        />
                      </ProFormGroup>
                      <ProFormDependency name={['algo_window', 'algo_k']}>
                        {({ algo_window, algo_k }) => (
                          <Card title="参数预览" size="small">
                            <div style={{ fontFamily: 'monospace' }}>
                              {JSON.stringify({ window: algo_window || '1h', k: Number(algo_k || 3) })}
                            </div>
                          </Card>
                        )}
                      </ProFormDependency>
                    </>
                  );
                }
                return (
                  <ProFormTextArea 
                    name="algo_params" 
                    label="算法参数 (JSON)" 
                    placeholder='{"window": "1h", "n": 3}'
                    initialValue='{"window": "1h", "n": 3}'
                    fieldProps={{ style: { fontFamily: 'monospace' } }}
                  />
                );
              }}
            </ProFormDependency>

            <ProFormGroup title="触发控制">
              <ProFormDigit
                name="eval_window_minutes"
                label="最近(分钟)"
                tooltip="用于 Prometheus：按窗口内均值判定；SQL 类数据源建议在 SQL 中自己做窗口聚合"
                min={0}
                initialValue={5}
                width="sm"
              />
              <ProFormDigit
                name="eval_consecutive"
                label="连续次数"
                tooltip="连续 N 次满足条件才报警（减少毛刺）"
                min={1}
                initialValue={1}
                width="sm"
              />
              <ProFormDigit name="duration" label="持续时间 (秒)" rules={[{ required: true }]} initialValue={60} width="sm" />
            </ProFormGroup>

            <Card title="告警升级" size="small" style={{ marginTop: 12 }}>
              <ProFormSwitch name="escalation_enabled" label="启用升级" initialValue={false} />
              <ProFormDependency name={['escalation_enabled', 'team_id']}>
                {({ escalation_enabled, team_id }) => {
                  if (!escalation_enabled) return null;
                  const opts = escalations
                    .filter((p) => !team_id || p.team_id === team_id)
                    .map((p) => ({ label: p.name, value: p.id }));
                  return (
                    <ProFormGroup>
                      <ProFormSelect
                        name="escalation_policy_id"
                        label="升级策略"
                        rules={[{ required: true }]}
                        options={opts}
                        width="xl"
                      />
                      <ProFormDigit
                        name="escalation_after"
                        label="触发次数后升级"
                        tooltip="连续处于触发状态时，发送第 N 次告警通知后启动升级策略"
                        initialValue={1}
                        min={1}
                        width="sm"
                      />
                    </ProFormGroup>
                  );
                }}
              </ProFormDependency>
            </Card>

            <ProFormDependency name={['data_name_id', 'eval_window_minutes']}>
              {({ data_name_id, eval_window_minutes }) => {
                const n = Number(eval_window_minutes || 0);
                if (!data_name_id || !n || n <= 0) return null;
                const dn = dataNames.find((d) => d.id === data_name_id);
                const t = dn?.data_source?.type;
                if (!t || t === 'prometheus') return null;
                const tf = dn?.time_field || (t === 'elasticsearch' ? '@timestamp' : 'ts');
                let example = '';
                if (t === 'mysql' || t === 'sqlserver') {
                  example = `WHERE ${tf} >= NOW() - INTERVAL ${n} MINUTE\nORDER BY ${tf} DESC\nLIMIT 100`;
                } else if (t === 'clickhouse') {
                  example = `WHERE ${tf} >= now() - INTERVAL ${n} MINUTE\nORDER BY ${tf} DESC\nLIMIT 100`;
                } else if (t === 'elasticsearch') {
                  example = `{\"query\":{\"bool\":{\"filter\":[{\"range\":{\"${tf}\":{\"gte\":\"now-${n}m\"}}}]}}}`;
                }
                if (!example) return null;
                return (
                  <Card title="时间窗口示例" size="small" style={{ marginTop: 12 }}>
                    <div style={{ fontFamily: 'monospace', whiteSpace: 'pre-wrap' }}>{example}</div>
                  </Card>
                );
              }}
            </ProFormDependency>
          </>
        )
      },
      {
        key: 'notify',
        label: '通知配置',
        forceRender: true,
        children: (
          <>
            <ProFormDependency name={['team_id']}>
              {({ team_id }) => {
                const opts = notificationChannels
                  .filter((c) => !team_id || c.team_id == null || c.team_id === team_id)
                  .map((c) => ({
                    label: `${c.type} · ${c.name}${c?.usage?.count ? `（引用${c.usage.count}）` : ''}`,
                    value: c.id,
                  }));
                return (
                  <ProFormSelect
                    name="channel_ids"
                    label="通知通道"
                    mode="multiple"
                    placeholder="优先使用通道统一管理 Webhook/SMTP 等配置"
                    options={opts}
                    fieldProps={{ showSearch: true, optionFilterProp: 'label' }}
                    width="xl"
                  />
                );
              }}
            </ProFormDependency>

            <ProFormSelect
              name="receivers"
              label="接收人"
              mode="multiple"
              options={users.map(u => ({ label: u.username, value: u.id }))}
              placeholder="选择接收人"
            />

            <ProFormSelect
              name="message_template_id"
              label="消息模板"
              options={templates.map((t) => ({ label: t.name, value: t.id }))}
              placeholder="选择模板 (留空使用默认)"
            />
            
            <ProFormDigit
              name="silence_period"
              label="通知冷却 (分钟)"
              tooltip="命中告警后在冷却时间内不重复发送通知；设为 0 表示不冷却（每次命中都会发送）"
              initialValue={5}
              min={0}
              width="sm"
            />
            <ProFormDependency name={['team_id']}>
              {({ team_id }) => {
                const opts = silences
                  .filter((s) => !team_id || s.team_id === team_id)
                  .map((s) => ({ label: s.name, value: s.id }));
                return (
                  <ProFormSelect
                    name="silence_rule_id"
                    label="高级静默"
                    placeholder="选择静默策略（可选）"
                    options={opts}
                    fieldProps={{ allowClear: true }}
                    width="xl"
                  />
                );
              }}
            </ProFormDependency>
            <ProFormSwitch name="notify_on_resolve" label="恢复通知" initialValue={true} />
            <ProFormText
              name="callback_url"
              label="回调地址"
              tooltip="告警触发/恢复/升级时将 POST JSON 到该地址"
              placeholder="https://example.com/argus/callback"
              width="xl"
            />
            <Card title="说明" size="small" style={{ marginTop: 12 }}>
              <div>告警规则优先使用“通知通道”发送；接收人用于钉钉 @（按用户手机号）。</div>
              <div>
                静默判断：先判断冷却静默（静默时间），再判断高级静默规则（
                <a onClick={() => navigate('/silences')}>管理</a>）。
              </div>
            </Card>
          </>
        )
      },
      {
        key: 'schedule',
        label: '调度配置',
        forceRender: true,
        children: (
          <>
            <ProFormSelect
              name="schedule_mode"
              label="调度方式"
              initialValue="every"
              options={[
                { label: '每 N 分钟', value: 'every' },
                { label: '每小时', value: 'hourly' },
                { label: '每天', value: 'daily' },
                { label: '每周', value: 'weekly' },
                { label: '每月', value: 'monthly' },
                { label: '高级 Cron', value: 'advanced' },
              ]}
              width="md"
            />

            <ProFormDependency name={['schedule_mode']}>
              {({ schedule_mode }) => {
                if (schedule_mode === 'every') {
                  return <ProFormDigit name="every_minutes" label="间隔(分钟)" initialValue={5} min={1} width="sm" />;
                }
                if (schedule_mode === 'hourly') {
                  return <ProFormDigit name="hourly_minute" label="每小时的第几分钟" initialValue={0} min={0} max={59} width="sm" />;
                }
                if (schedule_mode === 'daily') {
                  return <ProFormText name="daily_time" label="执行时间(HH:mm)" initialValue="03:00" width="sm" />;
                }
                if (schedule_mode === 'weekly') {
                  return (
                    <ProFormGroup>
                      <ProFormSelect
                        name="weekly_days"
                        label="周几"
                        mode="multiple"
                        options={[
                          { label: '周一', value: 1 },
                          { label: '周二', value: 2 },
                          { label: '周三', value: 3 },
                          { label: '周四', value: 4 },
                          { label: '周五', value: 5 },
                          { label: '周六', value: 6 },
                          { label: '周日', value: 7 },
                        ]}
                        width="md"
                      />
                      <ProFormText name="weekly_time" label="执行时间(HH:mm)" initialValue="03:00" width="sm" />
                    </ProFormGroup>
                  );
                }
                if (schedule_mode === 'monthly') {
                  return (
                    <ProFormGroup>
                      <ProFormSelect
                        name="monthly_days"
                        label="每月日期"
                        mode="multiple"
                        options={Array.from({ length: 31 }).map((_, i) => ({ label: String(i + 1), value: i + 1 }))}
                        width="md"
                      />
                      <ProFormText name="monthly_time" label="执行时间(HH:mm)" initialValue="03:00" width="sm" />
                    </ProFormGroup>
                  );
                }
                return (
                  <ProFormText 
                    name="cron_expression" 
                    label="Cron 表达式" 
                    placeholder="0 */5 * * * *"
                    width="lg"
                  />
                );
              }}
            </ProFormDependency>

            <ProFormDependency name={[
              'schedule_mode',
              'every_minutes',
              'hourly_minute',
              'daily_time',
              'weekly_days',
              'weekly_time',
              'monthly_days',
              'monthly_time',
              'cron_expression',
            ]}>
              {(vals) => (
                <Card title="Cron 预览" size="small" style={{ marginTop: 12 }}>
                  <div style={{ fontFamily: 'monospace' }}>{buildCronExpression(vals) || '-'}</div>
                </Card>
              )}
            </ProFormDependency>
            
            <Card title="生效时间窗口" size="small" style={{ marginTop: 16 }}>
              <ProFormSelect
                name="effective_mode"
                label="窗口类型"
                initialValue="none"
                options={[
                  { label: '不限制', value: 'none' },
                  { label: '按周', value: 'weekly' },
                  { label: '按月', value: 'monthly' },
                  { label: '按日期范围', value: 'range' },
                ]}
                width="md"
              />

              <ProFormDependency name={['effective_mode']}>
                {({ effective_mode }) => {
                  if (effective_mode === 'weekly') {
                    return (
                      <ProFormList name="weekly_windows" creatorButtonProps={{ creatorButtonText: '添加周窗口' }}>
                        <ProFormSelect
                          name="days"
                          label="周几"
                          mode="multiple"
                          options={[
                            { label: '周一', value: 1 },
                            { label: '周二', value: 2 },
                            { label: '周三', value: 3 },
                            { label: '周四', value: 4 },
                            { label: '周五', value: 5 },
                            { label: '周六', value: 6 },
                            { label: '周日', value: 7 },
                          ]}
                        />
                        <ProFormGroup>
                          <ProFormText name="start" label="开始(HH:mm)" initialValue="09:00" width="sm" />
                          <ProFormText name="end" label="结束(HH:mm)" initialValue="18:00" width="sm" />
                        </ProFormGroup>
                      </ProFormList>
                    );
                  }
                  if (effective_mode === 'monthly') {
                    return (
                      <ProFormList name="monthly_windows" creatorButtonProps={{ creatorButtonText: '添加月窗口' }}>
                        <ProFormSelect
                          name="days"
                          label="每月日期"
                          mode="multiple"
                          options={Array.from({ length: 31 }).map((_, i) => ({ label: String(i + 1), value: i + 1 }))}
                        />
                        <ProFormGroup>
                          <ProFormText name="start" label="开始(HH:mm)" initialValue="09:00" width="sm" />
                          <ProFormText name="end" label="结束(HH:mm)" initialValue="18:00" width="sm" />
                        </ProFormGroup>
                      </ProFormList>
                    );
                  }
                  if (effective_mode === 'range') {
                    return (
                      <ProFormList name="range_windows" creatorButtonProps={{ creatorButtonText: '添加日期范围窗口' }}>
                        <ProFormGroup>
                          <ProFormDatePicker name="startDate" label="开始日期" width="sm" />
                          <ProFormDatePicker name="endDate" label="结束日期" width="sm" />
                        </ProFormGroup>
                        <ProFormGroup>
                          <ProFormText name="start" label="开始(HH:mm)" initialValue="09:00" width="sm" />
                          <ProFormText name="end" label="结束(HH:mm)" initialValue="18:00" width="sm" />
                        </ProFormGroup>
                      </ProFormList>
                    );
                  }
                  return null;
                }}
              </ProFormDependency>
            </Card>
          </>
        )
      }
    ]} />
  );

  const prepareInitialValues = (record: any) => {
    let initial = { ...record };
    if (initial.notification_config) {
      try {
        const config = JSON.parse(initial.notification_config);
        initial.receivers = config.receivers || [];
      } catch (e) {}
    }
    if (initial.notification_props) {
      try {
        const props = JSON.parse(initial.notification_props);
        initial.channel_ids = props.channel_ids || [];
      } catch (e) {}
    }
    if (initial.effective_time) {
      try {
        const cfg = JSON.parse(initial.effective_time);
        initial.effective_mode = cfg.mode || 'none';
        if (cfg.mode === 'weekly') initial.weekly_windows = cfg.windows || [];
        if (cfg.mode === 'monthly') initial.monthly_windows = cfg.windows || [];
        if (cfg.mode === 'range') initial.range_windows = cfg.windows || [];
      } catch (e) {}
    }
    if (typeof initial.eval_window === 'string' && initial.eval_window.endsWith('m')) {
      const n = Number(initial.eval_window.slice(0, -1));
      if (!Number.isNaN(n)) {
        initial.eval_window_minutes = n;
      }
    } else {
      initial.eval_window_minutes = 0;
    }
    if (!initial.eval_consecutive) {
      initial.eval_consecutive = 1;
    }
    if (initial.algorithm === 'baseline_auto') {
      if (initial.algo_params) {
        try {
          const p = JSON.parse(initial.algo_params);
          if (p.sensitivity_k != null) initial.baseline_sensitivity_k = p.sensitivity_k;
          if (p.seasonality) initial.baseline_seasonality = p.seasonality;
        } catch (e) {}
      }
    }
    if (initial.algorithm === '3sigma' || initial.algorithm === 'mad') {
      if (initial.algo_params) {
        try {
          const p = JSON.parse(initial.algo_params);
          if (p.window) initial.algo_window = p.window;
          if (initial.algorithm === '3sigma' && p.n != null) initial.algo_n = p.n;
          if (initial.algorithm === 'mad' && p.k != null) initial.algo_k = p.k;
        } catch (e) {}
      }
    }

    const cron = typeof (initial.cron_expression || initial.cronExpression) === 'string'
      ? (initial.cron_expression || initial.cronExpression).trim()
      : '';
    if (cron) {
      const parts = cron.split(/\s+/);
      if (parts.length === 6) {
        const [sec, min, hour, dom, mon, dow] = parts;
        if (sec === '0' && hour === '*' && dom === '*' && mon === '*' && dow === '*' && /^\*\/\d+$/.test(min)) {
          initial.schedule_mode = 'every';
          initial.every_minutes = Number(min.replace('*/', '')) || 5;
        } else if (sec === '0' && hour === '*' && dom === '*' && mon === '*' && dow === '*' && /^\d+$/.test(min)) {
          initial.schedule_mode = 'hourly';
          initial.hourly_minute = Number(min) || 0;
        } else if (sec === '0' && dom === '*' && mon === '*' && dow === '*' && /^\d+$/.test(min) && /^\d+$/.test(hour)) {
          initial.schedule_mode = 'daily';
          const h = String(hour).padStart(2, '0');
          const m = String(min).padStart(2, '0');
          initial.daily_time = `${h}:${m}`;
        } else if (sec === '0' && dom === '*' && mon === '*' && /^\d+(,\d+)*$/.test(dow) && /^\d+$/.test(min) && /^\d+$/.test(hour)) {
          initial.schedule_mode = 'weekly';
          initial.weekly_days = dow.split(',').map((x: string) => Number(x)).filter((x: number) => !Number.isNaN(x));
          const h = String(hour).padStart(2, '0');
          const m = String(min).padStart(2, '0');
          initial.weekly_time = `${h}:${m}`;
        } else if (sec === '0' && mon === '*' && dow === '*' && /^\d+(,\d+)*$/.test(dom) && /^\d+$/.test(min) && /^\d+$/.test(hour)) {
          initial.schedule_mode = 'monthly';
          initial.monthly_days = dom.split(',').map((x: string) => Number(x)).filter((x: number) => !Number.isNaN(x));
          const h = String(hour).padStart(2, '0');
          const m = String(min).padStart(2, '0');
          initial.monthly_time = `${h}:${m}`;
        } else {
          initial.schedule_mode = 'advanced';
          initial.cron_expression = cron;
        }
      } else {
        initial.schedule_mode = 'advanced';
        initial.cron_expression = cron;
      }
    }
    return initial;
  };

  const handleFinish = async (values: any, isUpdate: boolean, id?: number) => {
    values = { ...(values || {}) };
    delete values.team;
    const dataName = dataNames.find((dn) => dn.id === values.data_name_id);
    const dsType = dataName?.data_source?.type;
    if (values.algorithm === 'baseline_auto' && dsType && dsType !== 'prometheus') {
      message.error('智能基线告警当前仅支持 Prometheus 数据源');
      return false;
    }
    if (dsType === 'prometheus') {
      values.value_mode = 'auto';
      values.row_pick = 'first';
      values.value_field = '';
    }
    if (dsType && dsType !== 'prometheus') {
      if (values.value_mode !== 'row_count' && !values.value_field) {
        message.error('SQL/ES/ClickHouse 等数据源请先选择“值字段”，或改为“返回行数”触发');
        return false;
      }
    }

    const channelIds = Array.isArray(values.channel_ids) ? values.channel_ids : [];
    const notification_props = JSON.stringify({ channel_ids: channelIds });

    // Construct notification_config
    const config = {
      receivers: values.receivers || [],
      webhooks: {},
    };

    let effective_time = values.effective_time;
    const mode = values.effective_mode || 'none';
    if (mode === 'weekly') {
      const windows = (values.weekly_windows || []).map((w: any) => ({ days: w.days || [], start: w.start, end: w.end }));
      effective_time = JSON.stringify({ mode, windows });
    } else if (mode === 'monthly') {
      const windows = (values.monthly_windows || []).map((w: any) => ({ days: w.days || [], start: w.start, end: w.end }));
      effective_time = JSON.stringify({ mode, windows });
    } else if (mode === 'range') {
      const windows = (values.range_windows || []).map((w: any) => ({ startDate: w.startDate, endDate: w.endDate, start: w.start, end: w.end }));
      effective_time = JSON.stringify({ mode, windows });
    } else if (mode === 'none') {
      effective_time = JSON.stringify({ mode: 'none', windows: [] });
    }

    let algo_params = values.algo_params;
    if (values.algorithm === 'mom' || values.algorithm === 'yoy') {
      const window = values.compare_window || '1h';
      const offset = values.compare_offset || (values.algorithm === 'yoy' ? '365d' : window);
      const change_percent = Number(values.compare_change_percent || 20);
      const direction = values.compare_direction || 'both';
      algo_params = JSON.stringify({ window, offset, change_percent, direction });
    }
    if (values.algorithm === '3sigma') {
      const window = values.algo_window || '1h';
      const n = Number(values.algo_n || 3);
      algo_params = JSON.stringify({ window, n });
    }
    if (values.algorithm === 'mad') {
      const window = values.algo_window || '1h';
      const k = Number(values.algo_k || 3);
      algo_params = JSON.stringify({ window, k });
    }
    if (values.algorithm === 'baseline_auto') {
      const sensitivity_k = Number(values.baseline_sensitivity_k || 3);
      const seasonality = values.baseline_seasonality || 'none';
      if (seasonality === 'daily') {
        algo_params = JSON.stringify({ sensitivity_k, seasonality, method: 'auto', training_days: 7 });
      } else if (seasonality === 'weekly') {
        algo_params = JSON.stringify({ sensitivity_k, seasonality, method: 'auto', training_days: 14 });
      } else {
        algo_params = JSON.stringify({ sensitivity_k, seasonality, method: 'auto', window: '3h' });
      }
      values.condition = values.condition || '>=';
      values.threshold = typeof values.threshold === 'number' ? values.threshold : 0;
    }

    const cron_expression = buildCronExpression(values) || values.cron_expression;
    if (values.schedule_mode && values.schedule_mode !== 'advanced') {
      if (!cron_expression) {
        message.error('调度配置不完整，无法生成 Cron 表达式');
        return false;
      }
    }

    let eval_window = values.eval_window;
    const winMin = Number(values.eval_window_minutes || 0);
    if (!Number.isNaN(winMin) && winMin > 0) {
      eval_window = `${winMin}m`;
    } else {
      eval_window = '0';
    }
    const eval_consecutive = Number(values.eval_consecutive || 1);
    const payload = {
      ...values,
      cron_expression,
      effective_time,
      eval_window,
      eval_consecutive,
      algo_params,
      notification_props,
      notification_config: JSON.stringify(config)
    };
    
    try {
      if (isUpdate && id) {
        await updateAlertRule(id, payload);
        message.success('更新成功');
      } else {
        await addAlertRule(payload);
        message.success('添加成功');
      }
      actionRef.current?.reload();
      return true;
    } catch (error) {
      message.error(isUpdate ? '更新失败' : '添加失败');
      return false;
    }
  };

  const columns: ProColumns[] = [
    {
      title: '规则名称',
      dataIndex: 'name',
      copyable: true,
      ellipsis: true,
      formItemProps: {
        rules: [{ required: true, message: '此项为必填项' }],
      },
    },
    {
      title: '团队',
      dataIndex: 'team_id',
      valueType: 'select',
      render: (_, record) => record.team?.name || '-',
      fieldProps: {
        options: teams.map((t) => ({ label: t.name, value: t.id })),
      },
    },
    {
      title: '告警级别',
      dataIndex: 'level',
      valueType: 'select',
      valueEnum: {
        critical: { text: '严重', status: 'Error' },
        warning: { text: '警告', status: 'Warning' },
        info: { text: '信息', status: 'Processing' },
      },
      formItemProps: {
        rules: [{ required: true, message: '此项为必填项' }],
      },
    },
    {
      title: '启用',
      dataIndex: 'is_enabled',
      valueType: 'select',
      valueEnum: {
        true: { text: '启用' },
        false: { text: '停用' },
      },
      render: (_, record) => (record.is_enabled ? '启用' : '停用'),
      hideInForm: true,
    },
    {
      title: '查询语句',
      dataIndex: 'query',
      valueType: 'textarea',
      hideInSearch: true,
      formItemProps: {
        rules: [{ required: true, message: '此项为必填项' }],
      },
    },
    {
      title: '持续时间(秒)',
      dataIndex: 'duration',
      hideInSearch: true,
      valueType: 'digit',
      formItemProps: {
        rules: [{ required: true, message: '此项为必填项' }],
      },
    },
    {
      title: '操作',
      valueType: 'option',
      render: (_text, record, _, _action) => [
        <a key="logs" onClick={() => navigate(`/alert-logs?alert_rule_id=${record.id}`)}>记录</a>,
        <a key="runtime" onClick={() => openRuntime(record.id)}>运行状态</a>,
        <ModalForm
          key="copy"
          title="复制告警规则"
          trigger={<a>复制</a>}
          formRef={copyFormRef}
          onFinish={(values) => {
            const merged: any = { ...prepareInitialValues(record), ...values };
            delete merged.id;
            return handleFinish(merged, false);
          }}
          onValuesChange={(_changedValues, values) => setCurrentFormValues(values)}
          submitter={{
            render: (props, dom) => {
              const submit = async () => {
                try {
                  await props.form?.validateFields?.();
                  props.submit?.();
                } catch (e: any) {
                  const errs = (e?.errorFields || []).flatMap((f: any) => f?.errors || []);
                  const labelMap: Record<string, string> = {
                    name: '规则名称',
                    team_id: '所属团队',
                    data_name_id: '数据名',
                    query: '查询语句',
                    duration: '持续时间(秒)',
                  };
                  const fields = (e?.errorFields || []).map((f: any) => labelMap[f?.name?.[0]] || f?.name?.[0]).filter(Boolean);
                  if (fields.includes('所属团队') || fields.includes('规则名称')) setActiveFormTab('basic');
                  else if (fields.includes('数据名') || fields.includes('查询语句')) setActiveFormTab('data');
                  else setActiveFormTab('basic');
                  Modal.warning({
                    title: '请完善必填项',
                    content: (
                      <div>
                        {fields.slice(0, 6).map((x: string) => (
                          <div key={x}>- {x}</div>
                        ))}
                        {errs?.[0] ? <div style={{ marginTop: 8 }}>{errs[0]}</div> : null}
                      </div>
                    ),
                  });
                }
              };
              return [
                dom?.[0],
                <Button key="submit" type="primary" onClick={submit}>
                  确定
                </Button>,
              ];
            },
          }}
          modalProps={{
            afterOpenChange: (open: boolean) => {
              if (open) {
                loadData();
                const initial = {
                  ...prepareInitialValues(record),
                  name: `${record.name}_副本`,
                  is_enabled: false,
                };
                copyFormRef.current?.resetFields?.();
                copyFormRef.current?.setFieldsValue?.(initial);
                setCurrentFormValues(initial);
                setActiveFormTab('basic');
              }
            }
          }}
        >
          {renderFormItems()}
        </ModalForm>,
        <Dropdown
          key="trigger"
          menu={{
            items: [
              { key: 'normal', label: '触发（遵循生效窗口）' },
              { key: 'force', label: '强制触发（跳过生效窗口）' },
            ],
            onClick: async ({ key }) => {
              try {
                await triggerAlertRule(record.id, key === 'force');
                message.success(key === 'force' ? '已强制触发' : '已触发');
              } catch (e) {
                message.error('触发失败');
              }
            },
          }}
        >
          <Tooltip title="触发：遵循生效窗口；强制触发：跳过生效窗口">
            <a>触发</a>
          </Tooltip>
        </Dropdown>,
        <ModalForm
          key="edit"
          title="编辑告警规则"
          trigger={<a>编辑</a>}
          onFinish={async () => {
            const all = editFormRef.current?.getFieldsValue?.(true) || {};
            return handleFinish(all, true, record.id);
          }}
          formRef={editFormRef}
          onValuesChange={(_changedValues, values) => setCurrentFormValues(values)}
          modalProps={{
            afterOpenChange: (open: boolean) => {
              if (open) {
                loadData();
                setActiveFormTab('basic');
                (async () => {
                  try {
                    const full = await getAlertRule(record.id);
                    const initial = prepareInitialValues(full);
                    editFormRef.current?.resetFields?.();
                    editFormRef.current?.setFieldsValue(initial);
                    setCurrentFormValues(initial);
                  } catch (e) {
                    const initial = prepareInitialValues(record);
                    editFormRef.current?.resetFields?.();
                    editFormRef.current?.setFieldsValue(initial);
                    setCurrentFormValues(initial);
                  }
                })();
              }
            }
          }}
        >
          {renderFormItems()}
        </ModalForm>,
        <Popconfirm
          key="delete"
          title="确定要删除吗？"
          onConfirm={async () => {
            try {
              await deleteAlertRule(record.id);
              message.success('删除成功');
              actionRef.current?.reload();
            } catch (error) {
              message.error('删除失败');
            }
          }}
        >
          <a>删除</a>
        </Popconfirm>,
      ],
    },
  ];

  const batchMenu = useMemo(() => {
    const ids = selectedRowKeys.map((k) => Number(k)).filter((x) => !Number.isNaN(x));
    const run = async (enabled: boolean) => {
      if (ids.length === 0) return;
      try {
        await batchUpdateAlertRulesEnabled(ids, enabled);
        message.success('批量更新成功');
        setSelectedRowKeys([]);
        actionRef.current?.reload();
      } catch (e) {
        message.error('批量更新失败');
      }
    };
    return {
      items: [
        { key: 'enable', label: '批量启用' },
        { key: 'disable', label: '批量停用' },
      ],
      onClick: ({ key }: any) => {
        if (key === 'enable') run(true);
        if (key === 'disable') run(false);
      },
    };
  }, [selectedRowKeys]);

  return (
    <>
      <ProTable
        columns={columns}
        actionRef={actionRef}
        cardBordered
        rowSelection={{
          selectedRowKeys,
          onChange: (keys) => setSelectedRowKeys(keys),
        }}
        request={async (params) => {
          try {
            await loadData();
            const res = await getAlertRules(params);
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
        headerTitle="告警规则列表"
        toolBarRender={() => [
          <Button
            key="create_btn"
            type="primary"
            onClick={() => {
              setCreateInitialValues({});
              setCreateOpen(true);
              setCurrentFormValues({});
              setActiveFormTab('basic');
              loadData();
            }}
          >
            <PlusOutlined />
            新建
          </Button>,
          <Dropdown key="batch" menu={batchMenu} disabled={selectedRowKeys.length === 0}>
            <Button>批量操作</Button>
          </Dropdown>,
          <ModalForm
            key="create_modal"
            title="新建告警规则"
            open={createOpen}
            formRef={createFormRef}
            onOpenChange={(open) => {
              setCreateOpen(open);
              if (open) {
                loadData();
                setCurrentFormValues(createInitialValues || {});
                setActiveFormTab('basic');
                setTimeout(() => {
                  createFormRef.current?.resetFields?.();
                  createFormRef.current?.setFieldsValue?.(createInitialValues || {});
                }, 0);
              }
            }}
            onFinish={async () => {
              const all = createFormRef.current?.getFieldsValue?.(true) || {};
              return handleFinish(all, false);
            }}
            onValuesChange={(_changedValues, values) => setCurrentFormValues(values)}
            submitter={{
              render: (props, dom) => {
                const submit = async () => {
                  try {
                    await props.form?.validateFields?.();
                    props.submit?.();
                  } catch (e: any) {
                    const errs = (e?.errorFields || []).flatMap((f: any) => f?.errors || []);
                    const labelMap: Record<string, string> = {
                      name: '规则名称',
                      team_id: '所属团队',
                      data_name_id: '数据名',
                      query: '查询语句',
                      duration: '持续时间(秒)',
                    };
                    const fields = (e?.errorFields || []).map((f: any) => labelMap[f?.name?.[0]] || f?.name?.[0]).filter(Boolean);
                    if (fields.includes('所属团队') || fields.includes('规则名称')) setActiveFormTab('basic');
                    else if (fields.includes('数据名') || fields.includes('查询语句')) setActiveFormTab('data');
                    else setActiveFormTab('basic');
                    Modal.warning({
                      title: '请完善必填项',
                      content: (
                        <div>
                          {fields.slice(0, 6).map((x: string) => (
                            <div key={x}>- {x}</div>
                          ))}
                          {errs?.[0] ? <div style={{ marginTop: 8 }}>{errs[0]}</div> : null}
                        </div>
                      ),
                    });
                  }
                };
                return [
                  dom?.[0],
                  <Button key="submit" type="primary" onClick={submit}>
                    确定
                  </Button>,
                ];
              },
            }}
          >
            {renderFormItems()}
          </ModalForm>,
        ]}
      />
      
      <Modal
        title="数据预览"
        open={previewModalVisible}
        onCancel={() => setPreviewModalVisible(false)}
        footer={null}
        width={800}
      >
        <div style={{ marginBottom: 12, display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          <Button
            onClick={async () => {
              const cols = Array.isArray(previewColumns) ? previewColumns : [];
              const tpl = buildTemplateSuggestion(cols);
              try {
                await navigator.clipboard.writeText(tpl);
                message.success('已复制模板建议到剪贴板');
              } catch (e) {
                message.error('复制失败，请手动复制');
              }
            }}
          >
            复制模板建议
          </Button>
          <ModalForm
            title="快速创建消息模板"
            trigger={<Button>快速创建消息模板</Button>}
            onFinish={async (values) => {
              try {
                await addTemplate(values);
                message.success('创建成功');
                loadData();
                return true;
              } catch (e) {
                message.error('创建失败（需要管理员权限）');
                return false;
              }
            }}
            initialValues={{
              name: `预览模板-${currentFormValues?.name || ''}`.trim() || '预览模板',
              type: 'general',
              format: 'text',
              content: buildTemplateSuggestion(Array.isArray(previewColumns) ? previewColumns : []),
              is_default: false,
            }}
          >
            <ProFormText name="name" label="模板名称" rules={[{ required: true }]} />
            <ProFormSelect
              name="type"
              label="类型"
              options={[
                { label: '通用', value: 'general' },
                { label: '邮件', value: 'email' },
                { label: '钉钉', value: 'dingtalk' },
                { label: '飞书', value: 'feishu' },
                { label: '企业微信', value: 'wechat' },
                { label: 'Slack', value: 'slack' },
                { label: 'Webhook', value: 'webhook' },
              ]}
            />
            <ProFormSelect
              name="format"
              label="格式"
              options={[
                { label: 'Text', value: 'text' },
                { label: 'Markdown', value: 'markdown' },
              ]}
              rules={[{ required: true }]}
            />
            <ProFormTextArea name="content" label="模板内容" fieldProps={{ rows: 10 }} rules={[{ required: true }]} />
            <ProFormSwitch name="is_default" label="设为默认" />
          </ModalForm>
          {previewColumns.length > 0 && (
            <Button
              onClick={async () => {
                const cols = previewColumns.join(', ');
                try {
                  await navigator.clipboard.writeText(cols);
                  message.success('已复制字段列表到剪贴板');
                } catch (e) {
                  message.error('复制失败，请手动复制');
                }
              }}
            >
              复制字段列表
            </Button>
          )}
        </div>
        <Table 
          dataSource={previewData} 
          columns={
            previewData.length > 0 
            ? Object.keys(previewData[0]).map(key => ({ title: key, dataIndex: key, key }))
            : []
          } 
          scroll={{ x: true }}
          pagination={false}
          size="small"
        />
      </Modal>

      <Modal
        title="运行状态"
        open={runtimeOpen}
        onCancel={() => setRuntimeOpen(false)}
        footer={null}
        width={900}
      >
        {runtimeLoading ? (
          <div>加载中...</div>
        ) : runtimeData ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            <Card size="small" title="调度与窗口">
              <div>当前时间：{runtimeData.now}</div>
              <div>启用：{runtimeData.is_enabled ? '是' : '否'}</div>
              <div>生效窗口：{runtimeData.effective_now ? '生效中' : '未生效'}</div>
              <div>Cron：{runtimeData.cron_expression}</div>
              {runtimeData.cron_error ? <div>Cron 错误：{runtimeData.cron_error}</div> : null}
              <div>
                下次执行：
                {Array.isArray(runtimeData.next_times) && runtimeData.next_times.length > 0 ? runtimeData.next_times[0] : '-'}
              </div>
              <div>
                后续执行：
                {Array.isArray(runtimeData.next_times) && runtimeData.next_times.length > 0
                  ? runtimeData.next_times.join(' | ')
                  : '-'}
              </div>
              <div>最近触发记录：{runtimeData.last_log_at || '-'}</div>
              <div>调度器 TTL(秒)：{runtimeData.leader_ttl_seconds ?? '-'}</div>
              <div>队列长度：{runtimeData.queue_len ?? '-'}</div>
            </Card>

            <Card size="small" title="告警与升级">
              <div>升级开关：{runtimeData.escalation_enabled ? '开启' : '关闭'}</div>
              <div>升级策略：{runtimeData.escalation_policy_id || '-'}</div>
              <Table
                rowKey="id"
                size="small"
                pagination={false}
                dataSource={runtimeData.alarms || []}
                columns={[
                  { title: 'AlarmID', dataIndex: 'id', width: 90 },
                  { title: '状态', dataIndex: 'status', width: 90 },
                  { title: '触发时间', dataIndex: 'starts_at', width: 190 },
                  { title: '静默剩余(秒)', dataIndex: 'silence_ttl_seconds', width: 120 },
                  { title: '升级剩余(秒)', dataIndex: 'escalation_ttl_seconds', width: 120 },
                  { title: '指纹', dataIndex: 'fingerprint', ellipsis: true },
                ]}
              />
            </Card>

            <Card size="small" title="评估记录（最近 20 次）">
              {evalLoading ? (
                <div>加载中...</div>
              ) : (
                <Table
                  rowKey="id"
                  size="small"
                  pagination={false}
                  dataSource={evalData || []}
                  columns={[
                    { title: '开始时间', dataIndex: 'started_at', width: 190 },
                    { title: '状态', dataIndex: 'status', width: 180 },
                    { title: '序列数', dataIndex: 'series_count', width: 90 },
                    { title: '触发数', dataIndex: 'trigger_count', width: 90 },
                    { title: '耗时(ms)', dataIndex: 'duration_ms', width: 90 },
                    { title: '错误', dataIndex: 'error_message', ellipsis: true },
                  ]}
                />
              )}
            </Card>
          </div>
        ) : (
          <div>暂无数据</div>
        )}
      </Modal>
    </>
  );
};

export default AlertRule;
