import React, { useEffect, useRef, useState } from 'react';
import {
  ProTable,
  type ProColumns,
  type ActionType,
  ModalForm,
  ProFormDateTimePicker,
  ProFormDependency,
  ProFormList,
  ProFormSwitch,
  ProFormText,
  ProFormSelect,
} from '@ant-design/pro-components';
import { Button, Modal, message, Popconfirm, Typography } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import { addSilence, deleteSilence, getSilences, updateSilence } from '../../services/silence';
import { getTeams } from '../../services/team';
import { getRoutingRuleMatcherOptions } from '../../services/routingRule';

const { Text } = Typography;

const SilenceList: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [teams, setTeams] = useState<any[]>([]);
  const [matcherOptions, setMatcherOptions] = useState<any>({ fields: [], levels: [], statuses: [], services: [], apps: [] });
  const [matcherOptionsByTeam, setMatcherOptionsByTeam] = useState<Record<number, any>>({});

  const ensureMatcherOptions = async (teamId?: number) => {
    const tid = Number(teamId || 0);
    if (tid > 0 && matcherOptionsByTeam[tid]) {
      setMatcherOptions(matcherOptionsByTeam[tid]);
      return;
    }
    try {
      const res: any = await getRoutingRuleMatcherOptions(tid > 0 ? { team_id: tid } : undefined);
      const next = res || { fields: [], levels: [], statuses: [], services: [], apps: [] };
      setMatcherOptions(next);
      if (tid > 0) setMatcherOptionsByTeam((prev) => ({ ...prev, [tid]: next }));
    } catch (e) {
      setMatcherOptions({ fields: [], levels: [], statuses: [], services: [], apps: [] });
    }
  };

  const toISO = (v: any) => {
    if (!v) return v;
    if (typeof v?.toDate === 'function') return v.toDate().toISOString();
    if (v instanceof Date) return v.toISOString();
    if (typeof v === 'string') {
      if (v.includes('T')) return v;
      if (v.includes(' ')) return `${v.replace(' ', 'T')}Z`;
      return v;
    }
    return v;
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
        const data = Array.isArray(res) ? res : (res.data || []);
        setTeams(data);
      } catch (e) {}
    })();
  }, []);

  useEffect(() => {
    (async () => {
      await ensureMatcherOptions();
    })();
  }, []);

  const columns: ProColumns[] = [
    {
      title: '名称',
      dataIndex: 'name',
      ellipsis: true,
    },
    {
      title: '团队',
      dataIndex: 'team_id',
      valueType: 'select',
      fieldProps: { options: teams.map((t) => ({ label: t.name, value: t.id })) },
      render: (_, record) => teams.find((t) => t.id === record.team_id)?.name || '-',
    },
    {
      title: '开始时间',
      dataIndex: 'starts_at',
      valueType: 'dateTime',
      hideInSearch: true,
      width: 180,
    },
    {
      title: '结束时间',
      dataIndex: 'ends_at',
      valueType: 'dateTime',
      hideInSearch: true,
      width: 180,
    },
    {
      title: 'Matchers',
      dataIndex: 'matchers',
      hideInSearch: true,
      ellipsis: true,
      render: (_, record) => (
        <Text ellipsis={{ tooltip: record.matchers }} style={{ width: 520 }}>
          {record.matchers}
        </Text>
      ),
    },
    {
      title: '备注',
      dataIndex: 'comment',
      ellipsis: true,
    },
    {
      title: '操作',
      valueType: 'option',
      render: (_text, record) => [
        <ModalForm
          key={`edit-${record.id}`}
          title="编辑静默"
          trigger={<a>编辑</a>}
          onFinish={async (values) => {
            const matchers = (values.matchers || []).map((m: any) => ({
              name: m.name,
              value: m.value,
              isRegex: !!m.isRegex,
            }));
            if (matchers.length === 0) {
              const ok = await new Promise<boolean>((resolve) => {
                Modal.confirm({
                  title: '确认保存全团队静默？',
                  content: '未设置匹配条件时，该静默会对该团队下所有（引用该静默规则的）告警生效。',
                  onOk: () => resolve(true),
                  onCancel: () => resolve(false),
                });
              });
              if (!ok) return false;
            }
            try {
              await updateSilence(record.id, {
                name: values.name,
                team_id: record.team_id,
                starts_at: toISO(values.starts_at),
                ends_at: toISO(values.ends_at),
                comment: values.comment,
                matchers: JSON.stringify(matchers),
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
            starts_at: record.starts_at ? dayjs(record.starts_at) : undefined,
            ends_at: record.ends_at ? dayjs(record.ends_at) : undefined,
            comment: record.comment,
            matchers: parseMatchers(record.matchers),
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
          <ProFormDateTimePicker name="starts_at" label="开始时间" rules={[{ required: true }]} />
          <ProFormDateTimePicker name="ends_at" label="结束时间" rules={[{ required: true }]} />
          <ProFormText name="comment" label="备注" />
          <Text type="secondary">
            多个匹配条件为且关系（AND）：需要全部满足才静默；留空表示对该团队下所有（引用该静默规则的）告警生效；字段不存在则视为不匹配。
          </Text>
          <Text type="secondary">需要或关系（OR）时可用正则：例如 level 用正则 `(critical|warning)`。</Text>
          <Text type="secondary">匹配字段支持：alert_rule_id、team_id、level、rule_name、fingerprint、service、app、status，以及 labels 内出现过的 key。</Text>
          <ProFormList name="matchers" creatorButtonProps={{ creatorButtonText: '添加匹配条件' }}>
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
              await deleteSilence(record.id);
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
          const res = await getSilences(params);
          const data = Array.isArray(res) ? res : (res.data || []);
          return { data, success: true, total: data.length };
        } catch (e) {
          return { data: [], success: false };
        }
      }}
      rowKey="id"
      search={{ labelWidth: 'auto' }}
      pagination={{ pageSize: 20 }}
      dateFormatter="string"
      headerTitle="静默规则"
      toolBarRender={() => [
        <ModalForm
          key="create"
          title="新建静默"
          trigger={
            <Button type="primary">
              <PlusOutlined />
              新建
            </Button>
          }
          onFinish={async (values) => {
            const matchers = (values.matchers || []).map((m: any) => ({
              name: m.name,
              value: m.value,
              isRegex: !!m.isRegex,
            }));
            if (matchers.length === 0) {
              const ok = await new Promise<boolean>((resolve) => {
                Modal.confirm({
                  title: '确认创建全团队静默？',
                  content: '未设置匹配条件时，该静默会对该团队下所有（引用该静默规则的）告警生效。',
                  onOk: () => resolve(true),
                  onCancel: () => resolve(false),
                });
              });
              if (!ok) return false;
            }
            try {
              await addSilence({
                name: values.name,
                team_id: values.team_id,
                starts_at: toISO(values.starts_at),
                ends_at: toISO(values.ends_at),
                comment: values.comment,
                matchers: JSON.stringify(matchers),
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
            team_id: undefined,
            matchers: [],
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
          <ProFormDateTimePicker name="starts_at" label="开始时间" rules={[{ required: true }]} />
          <ProFormDateTimePicker name="ends_at" label="结束时间" rules={[{ required: true }]} />
          <ProFormText name="comment" label="备注" />
          <Text type="secondary">
            多个匹配条件为且关系（AND）：需要全部满足才静默；留空表示对该团队下所有（引用该静默规则的）告警生效；字段不存在则视为不匹配。
          </Text>
          <Text type="secondary">需要或关系（OR）时可用正则：例如 level 用正则 `(critical|warning)`。</Text>
          <Text type="secondary">匹配字段支持：alert_rule_id、team_id、level、rule_name、fingerprint、service、app、status，以及 labels 内出现过的 key。</Text>
          <ProFormList name="matchers" creatorButtonProps={{ creatorButtonText: '添加匹配条件' }}>
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

export default SilenceList;
