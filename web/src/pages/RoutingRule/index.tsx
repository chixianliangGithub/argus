import React, { useEffect, useRef, useState } from 'react';
import {
  ModalForm,
  ProFormDigit,
  ProFormDependency,
  ProFormList,
  ProFormSelect,
  ProFormSwitch,
  ProFormText,
  ProFormTextArea,
  ProTable,
  type ActionType,
  type ProColumns,
} from '@ant-design/pro-components';
import { Button, Modal, Popconfirm, Typography, message } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import {
  addRoutingRule,
  deleteRoutingRule,
  getRoutingRuleMatcherOptions,
  getRoutingRules,
  previewRoutingRule,
  updateRoutingRule,
} from '../../services/routingRule';
import { getTeams } from '../../services/team';
import { getNotificationChannels } from '../../services/notificationChannel';
import { getAlarms } from '../../services/alarm';

const { Text } = Typography;

const RoutingRuleList: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const previewFormRef = useRef<any>();
  const [teams, setTeams] = useState<any[]>([]);
  const [channelsByTeam, setChannelsByTeam] = useState<Record<number, any[]>>({});
  const [matcherOptions, setMatcherOptions] = useState<any>({ fields: [], levels: [], statuses: [], services: [], apps: [] });
  const [matcherOptionsByTeam, setMatcherOptionsByTeam] = useState<Record<number, any>>({});

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

  // notification_props 约定：
  // - channel_ids: 路由指定的通知通道
  // - override: 是否覆盖规则基础通道
  //   - true(默认)：命中路由后，仅发路由通道（覆盖）
  //   - false：命中路由后，规则通道与路由通道都会发（去重叠加）
  const parseNotificationProps = (raw: any) => {
    try {
      const v = typeof raw === 'string' ? JSON.parse(raw || '{}') : raw;
      const ids = Array.isArray(v?.channel_ids) ? v.channel_ids : [];
      const override = v?.override === false ? false : true;
      return { channelIds: ids, override };
    } catch (e) {
      return { channelIds: [], override: true };
    }
  };

  const buildProps = (channelIds: number[], override: boolean) => {
    return JSON.stringify({ channel_ids: channelIds || [], override: override !== false });
  };

  const ensureChannels = async (teamId: number) => {
    if (!teamId) return;
    if (channelsByTeam[teamId]) return;
    try {
      const res: any = await getNotificationChannels({ team_id: teamId });
      const data = Array.isArray(res) ? res : res.data || [];
      setChannelsByTeam((prev) => ({ ...prev, [teamId]: data }));
    } catch (e) {
      setChannelsByTeam((prev) => ({ ...prev, [teamId]: [] }));
    }
  };

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
    {
      title: 'Matchers',
      dataIndex: 'matchers',
      hideInSearch: true,
      ellipsis: true,
      render: (_, record: any) => (
        <Text ellipsis={{ tooltip: record.matchers }} style={{ width: 520 }}>
          {record.matchers}
        </Text>
      ),
    },
    {
      title: '操作',
      valueType: 'option',
      render: (_text, record: any) => [
        <ModalForm
          key={`edit-${record.id}`}
          title="编辑路由规则"
          trigger={<a>编辑</a>}
          onFinish={async (values) => {
            const matchers = (values.matchers || []).map((m: any) => ({ name: m.name, value: m.value, isRegex: !!m.isRegex }));
            try {
              await updateRoutingRule(record.id, {
                name: values.name,
                team_id: record.team_id,
                is_enabled: !!values.is_enabled,
                priority: values.priority || 0,
                matchers: JSON.stringify(matchers),
                notification_props: buildProps(values.channel_ids || [], values.override_channels !== false),
                notification_config: values.notification_config || '',
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
            matchers: parseMatchers(record.matchers),
            channel_ids: parseNotificationProps(record.notification_props).channelIds,
            override_channels: parseNotificationProps(record.notification_props).override,
            notification_config: record.notification_config || '',
          }}
          modalProps={{
            afterOpenChange: (open) => {
              if (open) {
                ensureChannels(record.team_id);
                ensureMatcherOptions(record.team_id);
              }
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
          <Text type="secondary">
            多个匹配条件为且关系（AND）：需要全部满足才命中；留空表示匹配全部告警；字段不存在则视为不匹配。
          </Text>
          <Text type="secondary">
            需要或关系（OR）时可用正则：例如 level 用正则 `(critical|warning)`；或复制两条路由规则分别匹配。
          </Text>
          <ProFormList name="matchers" creatorButtonProps={{ creatorButtonText: '添加匹配条件' }}>
            <ProFormSelect
              name="name"
              label="字段"
              showSearch
              rules={[{ required: true }]}
              options={(() => {
                const fields = Array.isArray(matcherOptions?.fields) ? matcherOptions.fields : [];
                const base = ['rule_name', 'level', 'status', 'service', 'app'];
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
          <ProFormSelect
            name="channel_ids"
            label="路由到通道"
            mode="multiple"
            options={(channelsByTeam[record.team_id] || []).map((c) => ({ label: `${c.name}(${c.type})`, value: c.id }))}
          />
          <ProFormSwitch name="override_channels" label="覆盖规则通道" />
          <Text type="secondary">
            开启：只发“路由到通道”；关闭：同时发送“规则通道 + 路由通道”（去重叠加）。
          </Text>
          <ProFormTextArea
            name="notification_config"
            label="通知配置(可选)"
            fieldProps={{ rows: 4 }}
            placeholder='JSON，例如 {"receivers":[1,2],"webhooks":{"webhook":["https://..."]}}'
          />
        </ModalForm>,
        <ModalForm
          key={`preview-${record.id}`}
          title="路由预览"
          trigger={<a>预览</a>}
          onFinish={async (values: any) => {
            try {
              const r: any = await previewRoutingRule({ alarm_id: Number(values.alarm_id) });
              Modal.info({
                title: '预览结果',
                width: 900,
                content: <pre style={{ margin: 0, whiteSpace: 'pre-wrap' }}>{JSON.stringify(r, null, 2)}</pre>,
              });
              return true;
            } catch (e: any) {
              const msg = e?.response?.data?.error || e?.message || '预览失败';
              message.error(msg);
              return false;
            }
          }}
          initialValues={{ alarm_id: undefined }}
          formRef={previewFormRef}
        >
          <ProFormSelect
            name="alarm_pick"
            label="选择告警"
            showSearch
            debounceTime={300}
            placeholder="选择一个最近告警，自动填充 Alarm ID"
            request={async (params) => {
              const keyword = String((params as any)?.keyWords || '').trim();
              const r: any = await getAlarms({ status: 'firing', current: 1, pageSize: 50 });
              const rows = Array.isArray(r) ? r : r.data || [];
              const filtered = keyword
                ? rows.filter((x: any) => {
                    const id = String(x?.id ?? '');
                    const rule = String(x?.rule_name ?? '');
                    const fp = String(x?.fingerprint ?? '');
                    const svc = String(x?.service ?? '');
                    const app = String(x?.app ?? '');
                    return [id, rule, fp, svc, app].some((s) => s.toLowerCase().includes(keyword.toLowerCase()));
                  })
                : rows;
              return filtered.map((x: any) => ({
                label: `#${x.id} ${x.rule_name || ''} ${x.service ? `service=${x.service}` : ''} ${x.app ? `app=${x.app}` : ''}`.trim(),
                value: x.id,
              }));
            }}
            fieldProps={{
              onChange: (v) => {
                const id = Number(v || 0);
                if (!id) return;
                previewFormRef.current?.setFieldsValue({ alarm_id: String(id) });
              },
              filterOption: false,
            }}
          />
          <ProFormText name="alarm_id" label="Alarm ID" rules={[{ required: true }]} placeholder="可在【告警列表】里复制 ID" />
          <Text type="secondary">
            输入一个现有告警ID，返回匹配到的路由规则与最终通知配置。告警 ID 可在【告警列表】的 ID 列复制。
          </Text>
        </ModalForm>,
        <Popconfirm
          key="delete"
          title="确定要删除吗？"
          onConfirm={async () => {
            try {
              await deleteRoutingRule(record.id);
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
          const res = await getRoutingRules(params);
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
      headerTitle="路由规则"
      toolBarRender={() => [
        <ModalForm
          key="create"
          title="新建路由规则"
          trigger={
            <Button type="primary">
              <PlusOutlined />
              新建
            </Button>
          }
          onFinish={async (values: any) => {
            const matchers = (values.matchers || []).map((m: any) => ({ name: m.name, value: m.value, isRegex: !!m.isRegex }));
            try {
              await addRoutingRule({
                name: values.name,
                team_id: values.team_id,
                is_enabled: !!values.is_enabled,
                priority: values.priority || 0,
                matchers: JSON.stringify(matchers),
                notification_props: buildProps(values.channel_ids || [], values.override_channels !== false),
                notification_config: values.notification_config || '',
              });
              message.success('添加成功');
              actionRef.current?.reload();
              return true;
            } catch (e) {
              message.error('添加失败');
              return false;
            }
          }}
          initialValues={{ is_enabled: true, priority: 0, matchers: [], channel_ids: [], override_channels: false }}
          modalProps={{
            afterOpenChange: (open) => {
              if (open) {
                const tid = teams?.[0]?.id;
                if (tid) ensureChannels(tid);
                if (tid) ensureMatcherOptions(tid);
              }
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
            fieldProps={{
              onChange: (v) => {
                const tid = Number(v);
                ensureChannels(tid);
                ensureMatcherOptions(tid);
              },
            }}
          />
          <ProFormSwitch name="is_enabled" label="启用" />
          <ProFormDigit name="priority" label="优先级" min={0} fieldProps={{ precision: 0 }} />
          <Text type="secondary">
            多个匹配条件为且关系（AND）：需要全部满足才命中；留空表示匹配全部告警；字段不存在则视为不匹配。
          </Text>
          <Text type="secondary">
            需要或关系（OR）时可用正则：例如 level 用正则 `(critical|warning)`；或复制两条路由规则分别匹配。
          </Text>
          <ProFormList name="matchers" creatorButtonProps={{ creatorButtonText: '添加匹配条件' }}>
            <ProFormSelect
              name="name"
              label="字段"
              showSearch
              rules={[{ required: true }]}
              options={(() => {
                const fields = Array.isArray(matcherOptions?.fields) ? matcherOptions.fields : [];
                const base = ['rule_name', 'level', 'status', 'service', 'app'];
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
          <ProFormSelect
            name="channel_ids"
            label="路由到通道"
            mode="multiple"
            options={Object.values(channelsByTeam).flat().map((c) => ({ label: `${c.name}(${c.type})`, value: c.id }))}
          />
          <ProFormSwitch name="override_channels" label="覆盖规则通道" />
          <Text type="secondary">
            开启：只发“路由到通道”；关闭：同时发送“规则通道 + 路由通道”（去重叠加）。
          </Text>
          <ProFormTextArea
            name="notification_config"
            label="通知配置(可选)"
            fieldProps={{ rows: 4 }}
            placeholder='JSON，例如 {"receivers":[1,2],"webhooks":{"webhook":["https://..."]}}'
          />
        </ModalForm>,
      ]}
    />
  );
};

export default RoutingRuleList;
