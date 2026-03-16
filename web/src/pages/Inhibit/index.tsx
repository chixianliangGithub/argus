import React, { useEffect, useRef, useState } from 'react';
import {
  ModalForm,
  ProFormDependency,
  ProFormList,
  ProFormSelect,
  ProFormSwitch,
  ProFormText,
  ProFormTextArea,
  ProFormDigit,
  ProTable,
  type ActionType,
  type ProColumns,
} from '@ant-design/pro-components';
import { Button, Modal, Popconfirm, Typography, message } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { addInhibitRule, deleteInhibitRule, getInhibitRules, updateInhibitRule } from '../../services/inhibit';
import { getTeams } from '../../services/team';
import { getRoutingRuleMatcherOptions } from '../../services/routingRule';

const { Text } = Typography;

const InhibitRuleList: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [teams, setTeams] = useState<any[]>([]);
  const [matcherOptions, setMatcherOptions] = useState<any>({ fields: [], levels: [], statuses: [], services: [], apps: [], label_keys: [] });
  const [matcherOptionsByTeam, setMatcherOptionsByTeam] = useState<Record<number, any>>({});

  const ensureMatcherOptions = async (teamId?: number) => {
    const tid = Number(teamId || 0);
    if (tid > 0 && matcherOptionsByTeam[tid]) {
      setMatcherOptions(matcherOptionsByTeam[tid]);
      return;
    }
    try {
      const res: any = await getRoutingRuleMatcherOptions(tid > 0 ? { team_id: tid } : undefined);
      const next = res || { fields: [], levels: [], statuses: [], services: [], apps: [], label_keys: [] };
      setMatcherOptions(next);
      if (tid > 0) setMatcherOptionsByTeam((prev) => ({ ...prev, [tid]: next }));
    } catch (e) {
      setMatcherOptions({ fields: [], levels: [], statuses: [], services: [], apps: [], label_keys: [] });
    }
  };

  const parseMatchers = (raw: any) => {
    try {
      const arr = typeof raw === 'string' ? JSON.parse(raw || '[]') : raw;
      if (!Array.isArray(arr)) return [];
      return arr
        .filter((m) => m && typeof m === 'object')
        .map((m) => ({ name: m.name, value: m.value, isRegex: !!m.isRegex }));
    } catch (e) {
      return [];
    }
  };

  useEffect(() => {
    (async () => {
      try {
        const res = await getTeams();
        const data = Array.isArray(res) ? res : res.data || [];
        setTeams(data);
      } catch (e) {}
    })();
  }, []);

  useEffect(() => {
    (async () => {
      await ensureMatcherOptions();
    })();
  }, []);

  const splitCSV = (raw: any) => {
    const s = String(raw || '').trim();
    if (!s) return [];
    return s
      .split(',')
      .map((x) => x.trim())
      .filter(Boolean);
  };

  const joinCSV = (arr: any) => {
    if (typeof arr === 'string') return arr;
    if (!Array.isArray(arr)) return '';
    return arr
      .map((x) => String(x || '').trim())
      .filter(Boolean)
      .join(',');
  };

  const confirmRisky = async (title: string, content: string) => {
    return new Promise<boolean>((resolve) => {
      Modal.confirm({
        title,
        content,
        onOk: () => resolve(true),
        onCancel: () => resolve(false),
      });
    });
  };

  const columns: ProColumns[] = [
    { title: '名称', dataIndex: 'name', ellipsis: true },
    {
      title: '团队',
      dataIndex: 'team_id',
      valueType: 'select',
      fieldProps: { options: teams.map((t) => ({ label: t.name, value: t.id })) },
      render: (_, record) => teams.find((t) => t.id === record.team_id)?.name || '-',
    },
    {
      title: '启用',
      dataIndex: 'is_enabled',
      valueType: 'switch',
      width: 90,
      render: (_, record: any) => (record.is_enabled ? <span>是</span> : <span>否</span>),
    },
    { title: '优先级', dataIndex: 'priority', width: 90, hideInSearch: true },
    { title: 'Equal', dataIndex: 'equal_labels', width: 200, ellipsis: true, hideInSearch: true },
    { title: '描述', dataIndex: 'description', ellipsis: true, hideInSearch: true },
    {
      title: '操作',
      valueType: 'option',
      render: (_text, record: any) => [
        <ModalForm
          key={`edit-${record.id}`}
          title="编辑抑制规则"
          trigger={<a>编辑</a>}
          onFinish={async (values) => {
            const source = (values.source || []).map((m: any) => ({ name: m.name, value: m.value, isRegex: !!m.isRegex }));
            const target = (values.target || []).map((m: any) => ({ name: m.name, value: m.value, isRegex: !!m.isRegex }));
            if (target.length === 0) {
              const ok = await confirmRisky('确认保存空目标匹配？', 'Target 为空会导致该团队下大范围告警可能被抑制。');
              if (!ok) return false;
            }
            if (source.length === 0) {
              const ok = await confirmRisky('确认保存空源匹配？', 'Source 为空会导致任何告警都可能成为“抑制源”。');
              if (!ok) return false;
            }
            try {
              await updateInhibitRule(record.id, {
                name: values.name,
                team_id: record.team_id,
                is_enabled: !!values.is_enabled,
                priority: values.priority || 0,
                equal_labels: joinCSV(values.equal_labels),
                description: values.description || '',
                source: JSON.stringify(source),
                target: JSON.stringify(target),
              });
              message.success('更新成功');
              actionRef.current?.reload();
              return true;
            } catch (e) {
              message.error('更新失败');
              return false;
            }
          }}
          initialValues={{
            name: record.name,
            team_id: record.team_id,
            is_enabled: record.is_enabled,
            priority: record.priority ?? 0,
            equal_labels: splitCSV(record.equal_labels),
            description: record.description || '',
            source: parseMatchers(record.source),
            target: parseMatchers(record.target),
          }}
          modalProps={{
            afterOpenChange: (open) => {
              if (open) ensureMatcherOptions(record.team_id);
            },
          }}
        >
          <ProFormText name="name" label="名称" rules={[{ required: true }]} />
          <ProFormSelect
            name="team_id"
            label="团队"
            rules={[{ required: true }]}
            options={teams.map((t) => ({ label: t.name, value: t.id }))}
            width="md"
            disabled
          />
          <ProFormSwitch name="is_enabled" label="启用" />
          <ProFormDigit name="priority" label="优先级" min={0} fieldProps={{ precision: 0 }} />
          <Text type="secondary">数值越大优先级越高；同团队按优先级从大到小匹配，命中第一条规则即生效。</Text>
          <ProFormSelect
            name="equal_labels"
            label="Equal Labels"
            mode="tags"
            placeholder="例如 service,cluster"
            options={(Array.isArray(matcherOptions?.label_keys) ? matcherOptions.label_keys : []).map((k: any) => ({ label: k, value: k }))}
          />
          <Text type="secondary">Equal Labels 用于要求源告警与目标告警在这些标签上取值一致（用于“关联同一业务/同一集群”）。</Text>
          <ProFormTextArea name="description" label="描述" fieldProps={{ rows: 2 }} />
          <Text type="secondary">
            Source/Target 的多个匹配条件为且关系（AND）：需要全部满足才算命中；字段不存在则视为不匹配；需要或关系时可用正则 `(a|b)`。
          </Text>
          <ProFormList name="source" creatorButtonProps={{ creatorButtonText: '添加 Source 匹配' }}>
            <ProFormSelect
              name="name"
              label="字段"
              showSearch
              rules={[{ required: true }]}
              options={(() => {
                const fields = Array.isArray(matcherOptions?.fields) ? matcherOptions.fields : [];
                const base = ['alert_rule_id', 'team_id', 'fingerprint', 'rule_name', 'level', 'status', 'service', 'app'];
                const merged = Array.from(new Set([...base, ...fields]));
                return merged.map((x) => ({ label: x, value: x }));
              })()}
            />
            <ProFormDependency name={['isRegex', 'name']}>
              {({ isRegex, name }) => {
                const field = String(name || '');
                if (isRegex) {
                  return <ProFormText name="value" label="值" rules={[{ required: true }]} placeholder="正则表达式" />;
                }
                if (field === 'level') {
                  const opts = (Array.isArray(matcherOptions?.levels) ? matcherOptions.levels : ['critical', 'warning', 'info']).map((x: any) => ({
                    label: x,
                    value: x,
                  }));
                  return <ProFormSelect name="value" label="值" rules={[{ required: true }]} options={opts} />;
                }
                if (field === 'status') {
                  const opts = (Array.isArray(matcherOptions?.statuses) ? matcherOptions.statuses : ['firing', 'resolved']).map((x: any) => ({
                    label: x,
                    value: x,
                  }));
                  return <ProFormSelect name="value" label="值" rules={[{ required: true }]} options={opts} />;
                }
                if (field === 'service') {
                  const opts = (Array.isArray(matcherOptions?.services) ? matcherOptions.services : []).map((x: any) => ({ label: x, value: x }));
                  if (opts.length > 0) return <ProFormSelect name="value" label="值" rules={[{ required: true }]} options={opts} showSearch />;
                }
                if (field === 'app') {
                  const opts = (Array.isArray(matcherOptions?.apps) ? matcherOptions.apps : []).map((x: any) => ({ label: x, value: x }));
                  if (opts.length > 0) return <ProFormSelect name="value" label="值" rules={[{ required: true }]} options={opts} showSearch />;
                }
                return <ProFormText name="value" label="值" rules={[{ required: true }]} />;
              }}
            </ProFormDependency>
            <ProFormSwitch name="isRegex" label="正则" initialValue={false} />
          </ProFormList>
          <ProFormList name="target" creatorButtonProps={{ creatorButtonText: '添加 Target 匹配' }}>
            <ProFormSelect
              name="name"
              label="字段"
              showSearch
              rules={[{ required: true }]}
              options={(() => {
                const fields = Array.isArray(matcherOptions?.fields) ? matcherOptions.fields : [];
                const base = ['alert_rule_id', 'team_id', 'fingerprint', 'rule_name', 'level', 'status', 'service', 'app'];
                const merged = Array.from(new Set([...base, ...fields]));
                return merged.map((x) => ({ label: x, value: x }));
              })()}
            />
            <ProFormDependency name={['isRegex', 'name']}>
              {({ isRegex, name }) => {
                const field = String(name || '');
                if (isRegex) {
                  return <ProFormText name="value" label="值" rules={[{ required: true }]} placeholder="正则表达式" />;
                }
                if (field === 'level') {
                  const opts = (Array.isArray(matcherOptions?.levels) ? matcherOptions.levels : ['critical', 'warning', 'info']).map((x: any) => ({
                    label: x,
                    value: x,
                  }));
                  return <ProFormSelect name="value" label="值" rules={[{ required: true }]} options={opts} />;
                }
                if (field === 'status') {
                  const opts = (Array.isArray(matcherOptions?.statuses) ? matcherOptions.statuses : ['firing', 'resolved']).map((x: any) => ({
                    label: x,
                    value: x,
                  }));
                  return <ProFormSelect name="value" label="值" rules={[{ required: true }]} options={opts} />;
                }
                if (field === 'service') {
                  const opts = (Array.isArray(matcherOptions?.services) ? matcherOptions.services : []).map((x: any) => ({ label: x, value: x }));
                  if (opts.length > 0) return <ProFormSelect name="value" label="值" rules={[{ required: true }]} options={opts} showSearch />;
                }
                if (field === 'app') {
                  const opts = (Array.isArray(matcherOptions?.apps) ? matcherOptions.apps : []).map((x: any) => ({ label: x, value: x }));
                  if (opts.length > 0) return <ProFormSelect name="value" label="值" rules={[{ required: true }]} options={opts} showSearch />;
                }
                return <ProFormText name="value" label="值" rules={[{ required: true }]} />;
              }}
            </ProFormDependency>
            <ProFormSwitch name="isRegex" label="正则" initialValue={false} />
          </ProFormList>
        </ModalForm>,
        <Popconfirm
          key="delete"
          title="确定要删除吗？"
          onConfirm={async () => {
            try {
              await deleteInhibitRule(record.id);
              message.success('删除成功');
              actionRef.current?.reload();
            } catch (e) {
              message.error('删除失败');
            }
          }}
        >
          <a>删除</a>
        </Popconfirm>,
      ],
    },
  ];

  return (
    <ProTable
      columns={columns}
      actionRef={actionRef}
      cardBordered
      request={async (params) => {
        try {
          const res = await getInhibitRules(params);
          const data = Array.isArray(res) ? res : res.data || [];
          return { data, success: true, total: data.length };
        } catch (e) {
          return { data: [], success: false };
        }
      }}
      rowKey="id"
      search={{ labelWidth: 'auto' }}
      pagination={{ pageSize: 20 }}
      dateFormatter="string"
      headerTitle="抑制规则"
      toolBarRender={() => [
        <ModalForm
          key="create"
          title="新建抑制规则"
          trigger={
            <Button type="primary">
              <PlusOutlined />
              新建
            </Button>
          }
          onFinish={async (values) => {
            const source = (values.source || []).map((m: any) => ({ name: m.name, value: m.value, isRegex: !!m.isRegex }));
            const target = (values.target || []).map((m: any) => ({ name: m.name, value: m.value, isRegex: !!m.isRegex }));
            if (target.length === 0) {
              const ok = await confirmRisky('确认创建空目标匹配？', 'Target 为空会导致该团队下大范围告警可能被抑制。');
              if (!ok) return false;
            }
            if (source.length === 0) {
              const ok = await confirmRisky('确认创建空源匹配？', 'Source 为空会导致任何告警都可能成为“抑制源”。');
              if (!ok) return false;
            }
            try {
              await addInhibitRule({
                name: values.name,
                team_id: values.team_id,
                is_enabled: !!values.is_enabled,
                priority: values.priority || 0,
                equal_labels: joinCSV(values.equal_labels),
                description: values.description || '',
                source: JSON.stringify(source),
                target: JSON.stringify(target),
              });
              message.success('添加成功');
              actionRef.current?.reload();
              return true;
            } catch (e) {
              message.error('添加失败');
              return false;
            }
          }}
          initialValues={{
            is_enabled: true,
            priority: 0,
            equal_labels: [],
            source: [],
            target: [],
          }}
        >
          <ProFormText name="name" label="名称" rules={[{ required: true }]} />
          <ProFormSelect
            name="team_id"
            label="团队"
            rules={[{ required: true }]}
            options={teams.map((t) => ({ label: t.name, value: t.id }))}
            width="md"
            fieldProps={{
              onChange: (v) => ensureMatcherOptions(Number(v)),
            }}
          />
          <ProFormSwitch name="is_enabled" label="启用" />
          <ProFormDigit name="priority" label="优先级" min={0} fieldProps={{ precision: 0 }} />
          <Text type="secondary">数值越大优先级越高；同团队按优先级从大到小匹配，命中第一条规则即生效。</Text>
          <ProFormSelect
            name="equal_labels"
            label="Equal Labels"
            mode="tags"
            placeholder="例如 service,cluster"
            options={(Array.isArray(matcherOptions?.label_keys) ? matcherOptions.label_keys : []).map((k: any) => ({ label: k, value: k }))}
          />
          <Text type="secondary">Equal Labels 用于要求源告警与目标告警在这些标签上取值一致（用于“关联同一业务/同一集群”）。</Text>
          <ProFormTextArea name="description" label="描述" fieldProps={{ rows: 2 }} />
          <Text type="secondary">
            Source/Target 的多个匹配条件为且关系（AND）：需要全部满足才算命中；字段不存在则视为不匹配；需要或关系时可用正则 `(a|b)`。
          </Text>
          <ProFormList name="source" creatorButtonProps={{ creatorButtonText: '添加 Source 匹配' }}>
            <ProFormSelect
              name="name"
              label="字段"
              showSearch
              rules={[{ required: true }]}
              options={(() => {
                const fields = Array.isArray(matcherOptions?.fields) ? matcherOptions.fields : [];
                const base = ['alert_rule_id', 'team_id', 'fingerprint', 'rule_name', 'level', 'status', 'service', 'app'];
                const merged = Array.from(new Set([...base, ...fields]));
                return merged.map((x) => ({ label: x, value: x }));
              })()}
            />
            <ProFormDependency name={['isRegex', 'name']}>
              {({ isRegex, name }) => {
                const field = String(name || '');
                if (isRegex) {
                  return <ProFormText name="value" label="值" rules={[{ required: true }]} placeholder="正则表达式" />;
                }
                if (field === 'level') {
                  const opts = (Array.isArray(matcherOptions?.levels) ? matcherOptions.levels : ['critical', 'warning', 'info']).map((x: any) => ({
                    label: x,
                    value: x,
                  }));
                  return <ProFormSelect name="value" label="值" rules={[{ required: true }]} options={opts} />;
                }
                if (field === 'status') {
                  const opts = (Array.isArray(matcherOptions?.statuses) ? matcherOptions.statuses : ['firing', 'resolved']).map((x: any) => ({
                    label: x,
                    value: x,
                  }));
                  return <ProFormSelect name="value" label="值" rules={[{ required: true }]} options={opts} />;
                }
                if (field === 'service') {
                  const opts = (Array.isArray(matcherOptions?.services) ? matcherOptions.services : []).map((x: any) => ({ label: x, value: x }));
                  if (opts.length > 0) return <ProFormSelect name="value" label="值" rules={[{ required: true }]} options={opts} showSearch />;
                }
                if (field === 'app') {
                  const opts = (Array.isArray(matcherOptions?.apps) ? matcherOptions.apps : []).map((x: any) => ({ label: x, value: x }));
                  if (opts.length > 0) return <ProFormSelect name="value" label="值" rules={[{ required: true }]} options={opts} showSearch />;
                }
                return <ProFormText name="value" label="值" rules={[{ required: true }]} />;
              }}
            </ProFormDependency>
            <ProFormSwitch name="isRegex" label="正则" initialValue={false} />
          </ProFormList>
          <ProFormList name="target" creatorButtonProps={{ creatorButtonText: '添加 Target 匹配' }}>
            <ProFormSelect
              name="name"
              label="字段"
              showSearch
              rules={[{ required: true }]}
              options={(() => {
                const fields = Array.isArray(matcherOptions?.fields) ? matcherOptions.fields : [];
                const base = ['alert_rule_id', 'team_id', 'fingerprint', 'rule_name', 'level', 'status', 'service', 'app'];
                const merged = Array.from(new Set([...base, ...fields]));
                return merged.map((x) => ({ label: x, value: x }));
              })()}
            />
            <ProFormDependency name={['isRegex', 'name']}>
              {({ isRegex, name }) => {
                const field = String(name || '');
                if (isRegex) {
                  return <ProFormText name="value" label="值" rules={[{ required: true }]} placeholder="正则表达式" />;
                }
                if (field === 'level') {
                  const opts = (Array.isArray(matcherOptions?.levels) ? matcherOptions.levels : ['critical', 'warning', 'info']).map((x: any) => ({
                    label: x,
                    value: x,
                  }));
                  return <ProFormSelect name="value" label="值" rules={[{ required: true }]} options={opts} />;
                }
                if (field === 'status') {
                  const opts = (Array.isArray(matcherOptions?.statuses) ? matcherOptions.statuses : ['firing', 'resolved']).map((x: any) => ({
                    label: x,
                    value: x,
                  }));
                  return <ProFormSelect name="value" label="值" rules={[{ required: true }]} options={opts} />;
                }
                if (field === 'service') {
                  const opts = (Array.isArray(matcherOptions?.services) ? matcherOptions.services : []).map((x: any) => ({ label: x, value: x }));
                  if (opts.length > 0) return <ProFormSelect name="value" label="值" rules={[{ required: true }]} options={opts} showSearch />;
                }
                if (field === 'app') {
                  const opts = (Array.isArray(matcherOptions?.apps) ? matcherOptions.apps : []).map((x: any) => ({ label: x, value: x }));
                  if (opts.length > 0) return <ProFormSelect name="value" label="值" rules={[{ required: true }]} options={opts} showSearch />;
                }
                return <ProFormText name="value" label="值" rules={[{ required: true }]} />;
              }}
            </ProFormDependency>
            <ProFormSwitch name="isRegex" label="正则" initialValue={false} />
          </ProFormList>
        </ModalForm>,
      ]}
    />
  );
};

export default InhibitRuleList;
