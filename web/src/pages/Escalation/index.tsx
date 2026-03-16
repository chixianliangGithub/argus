import React, { useEffect, useMemo, useRef, useState } from 'react';
import { ProTable, type ProColumns, type ActionType, ModalForm, ProFormDigit, ProFormGroup, ProFormList, ProFormSelect, ProFormText, ProFormDependency } from '@ant-design/pro-components';
import { Button, Card, message, Popconfirm, Tag, Typography } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { addEscalation, deleteEscalation, getEscalations, updateEscalation } from '../../services/escalation';
import { getTeams } from '../../services/team';
import { getNotificationChannels } from '../../services/notificationChannel';

const { Text } = Typography;

const EscalationList: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [teams, setTeams] = useState<any[]>([]);
  const [channels, setChannels] = useState<any[]>([]);

  useEffect(() => {
    (async () => {
      try {
        const [tRes, cRes] = await Promise.all([getTeams(), getNotificationChannels()]);
        setTeams(Array.isArray(tRes) ? tRes : (tRes.data || []));
        setChannels(Array.isArray(cRes) ? cRes : (cRes.data || []));
      } catch (e) {}
    })();
  }, []);

  const buildSubmit = (values: any) => {
    const steps = (values.steps || []).map((s: any) => ({
      wait_minutes: Number(s.wait_minutes || 0),
      channel_ids: Array.isArray(s.channel_ids) ? s.channel_ids : [],
    }));
    return {
      name: values.name,
      team_id: values.team_id,
      repeat_times: Number(values.repeat_times || 0),
      steps: JSON.stringify(steps),
    };
  };

  const parseRecord = (record: any) => {
    let steps: any[] = [];
    try {
      steps = JSON.parse(record.steps || '[]');
    } catch (e) {}
    return { ...record, steps };
  };

  const renderHelp = () => (
    <ProFormDependency name={['repeat_times', 'steps']}>
      {({ repeat_times, steps }) => {
        const repeats = Number(repeat_times || 0);
        const ss = Array.isArray(steps) ? steps : [];
        const lines: string[] = [];
        let offset = 0;
        let idx = 0;
        for (let r = 0; r <= Math.max(0, repeats); r++) {
          for (const s of ss) {
            const w = Number(s?.wait_minutes || 0);
            offset += Math.max(0, w);
            idx += 1;
            const ch = Array.isArray(s?.channel_ids) ? s.channel_ids.join(',') : '';
            lines.push(`第${r + 1}轮·第${idx}步：+${offset} 分钟 -> 通知通道(${ch || '-'})`);
            if (lines.length >= 12) break;
          }
          if (lines.length >= 12) break;
        }
        return (
          <Card size="small" title="说明" style={{ marginTop: 12 }}>
            <div>等待(分钟)：表示“从上一条升级通知发送后，再等待 N 分钟才执行本步骤”。</div>
            <div>重复次数：表示“整个步骤列表再重复 N 次”（0 表示只跑 1 轮）。</div>
            <div style={{ marginTop: 8 }}>执行预览（最多显示 12 条）：</div>
            <div style={{ marginTop: 4, fontFamily: 'monospace' }}>{lines.length ? lines.join('\n') : '-'}</div>
          </Card>
        );
      }}
    </ProFormDependency>
  );

  const columns: ProColumns[] = useMemo(() => {
    return [
      {
        title: '名称',
        dataIndex: 'name',
        copyable: true,
        ellipsis: true,
        formItemProps: { rules: [{ required: true, message: '此项为必填项' }] },
      },
      {
        title: '团队',
        dataIndex: 'team_id',
        valueType: 'select',
        fieldProps: { options: teams.map((t) => ({ label: t.name, value: t.id })) },
        render: (_, record) => {
          const t = teams.find((x) => x.id === record.team_id);
          return t?.name || '-';
        },
      },
      {
        title: '重复次数',
        dataIndex: 'repeat_times',
        hideInSearch: true,
        render: (_, record) => <Tag>{record.repeat_times || 0}</Tag>,
      },
      {
        title: '步骤',
        dataIndex: 'steps',
        hideInSearch: true,
        render: (_, record) => {
          let steps: any[] = [];
          try {
            steps = JSON.parse(record.steps || '[]');
          } catch (e) {}
          const text = steps
            .map((s) => `${s.wait_minutes}m -> ${Array.isArray(s.channel_ids) ? s.channel_ids.join(',') : ''}`)
            .join(' | ');
          return (
            <Text ellipsis={{ tooltip: text }} style={{ width: 520 }}>
              {text || '-'}
            </Text>
          );
        },
      },
      {
        title: '操作',
        valueType: 'option',
        render: (_text, record) => [
          <ModalForm
            key="edit"
            title="编辑升级策略"
            trigger={<a>编辑</a>}
            initialValues={parseRecord(record)}
            onFinish={async (values) => {
              try {
                await updateEscalation(record.id, buildSubmit(values));
                message.success('更新成功');
                actionRef.current?.reload();
                return true;
              } catch (e) {
                message.error('更新失败');
                return false;
              }
            }}
          >
            <ProFormText name="name" label="名称" rules={[{ required: true }]} />
            <ProFormSelect
              name="team_id"
              label="团队"
              rules={[{ required: true }]}
              options={teams.map((t) => ({ label: t.name, value: t.id }))}
              width="md"
            />
            <ProFormDigit name="repeat_times" label="重复次数" initialValue={0} min={0} width="sm" />
            <ProFormList name="steps" creatorButtonProps={{ creatorButtonText: '添加升级步骤' }}>
              <ProFormGroup>
                <ProFormDigit name="wait_minutes" label="等待(分钟)" min={0} initialValue={10} width="sm" />
                <ProFormSelect
                  name="channel_ids"
                  label="通知通道"
                  mode="multiple"
                  request={async () => {
                    const res = await getNotificationChannels();
                    const data = Array.isArray(res) ? res : (res.data || []);
                    return data.map((c: any) => ({ label: `${c.type} · ${c.name}`, value: c.id }));
                  }}
                  width="xl"
                />
              </ProFormGroup>
            </ProFormList>
            {renderHelp()}
          </ModalForm>,
          <Popconfirm
            key="delete"
            title="确定要删除吗？"
            onConfirm={async () => {
              try {
                await deleteEscalation(record.id);
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
  }, [teams, channels]);

  return (
    <ProTable
      columns={columns}
      actionRef={actionRef}
      cardBordered
      request={async (params) => {
        try {
          const res = await getEscalations(params);
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
      headerTitle="告警升级策略"
      toolBarRender={() => [
        <ModalForm
          key="create"
          title="新建升级策略"
          trigger={
            <Button type="primary">
              <PlusOutlined />
              新建
            </Button>
          }
          onFinish={async (values) => {
            try {
              await addEscalation(buildSubmit(values));
              message.success('添加成功');
              actionRef.current?.reload();
              return true;
            } catch (e) {
              message.error('添加失败');
              return false;
            }
          }}
          initialValues={{ repeat_times: 0, steps: [{ wait_minutes: 10, channel_ids: [] }] }}
        >
          <ProFormText name="name" label="名称" rules={[{ required: true }]} />
          <ProFormSelect
            name="team_id"
            label="团队"
            rules={[{ required: true }]}
            options={teams.map((t) => ({ label: t.name, value: t.id }))}
            width="md"
          />
          <ProFormDigit name="repeat_times" label="重复次数" initialValue={0} min={0} width="sm" />
          <ProFormList name="steps" creatorButtonProps={{ creatorButtonText: '添加升级步骤' }}>
            <ProFormGroup>
              <ProFormDigit name="wait_minutes" label="等待(分钟)" min={0} initialValue={10} width="sm" />
              <ProFormSelect
                name="channel_ids"
                label="通知通道"
                mode="multiple"
                request={async () => {
                  const res = await getNotificationChannels();
                  const data = Array.isArray(res) ? res : (res.data || []);
                  return data.map((c: any) => ({ label: `${c.type} · ${c.name}`, value: c.id }));
                }}
                width="xl"
              />
            </ProFormGroup>
          </ProFormList>
          {renderHelp()}
        </ModalForm>,
      ]}
    />
  );
};

export default EscalationList;
