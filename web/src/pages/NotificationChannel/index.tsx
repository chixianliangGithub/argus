import React, { useRef } from 'react';
import { ProTable, type ProColumns, type ActionType, ModalForm, ProFormDependency, ProFormSelect, ProFormText } from '@ant-design/pro-components';
import { Button, message, Popconfirm, Tag, Typography } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { addNotificationChannel, deleteNotificationChannel, getNotificationChannels, testNotificationChannel, updateNotificationChannel } from '../../services/notificationChannel';
import { getTeams } from '../../services/team';

const { Text } = Typography;

const NotificationChannelList: React.FC = () => {
  const actionRef = useRef<ActionType>();

  const buildSubmit = (values: any) => {
    const type = values.type;
    const cfg: any = {};
    if (type === 'dingtalk' || type === 'feishu' || type === 'wechat' || type === 'slack') {
      cfg.webhook_url = values.webhook_url;
    } else if (type === 'webhook') {
      cfg.url = values.url;
      if (values.token) cfg.token = values.token;
    } else if (type === 'email') {
      cfg.smtp_host = values.smtp_host;
      cfg.smtp_port = values.smtp_port;
      cfg.username = values.username;
      cfg.password = values.password;
      if (values.from) cfg.from = values.from;
      if (values.subject) cfg.subject = values.subject;
      if (values.tls_mode) cfg.tls_mode = values.tls_mode;
    }
    return {
      name: values.name,
      type,
      team_id: values.team_id,
      config: JSON.stringify(cfg),
    };
  };

  const parseRecord = (record: any) => {
    const cfgStr = record?.config || record?.Config || '';
    let cfg: any = {};
    if (cfgStr) {
      try {
        cfg = JSON.parse(cfgStr);
      } catch (e) {}
    }
    return {
      ...record,
      config: cfgStr,
      webhook_url: cfg.webhook_url,
      url: cfg.url,
      token: cfg.token,
      smtp_host: cfg.smtp_host,
      smtp_port: cfg.smtp_port,
      username: cfg.username,
      password: cfg.password,
      to: cfg.to,
      from: cfg.from,
      subject: cfg.subject,
      tls_mode: cfg.tls_mode || 'plain',
    };
  };

  const renderConfigFields = () => (
    <ProFormDependency name={['type']}>
      {({ type }) => {
        if (type === 'dingtalk' || type === 'feishu' || type === 'wechat' || type === 'slack') {
          return (
            <ProFormText
              name="webhook_url"
              label="Webhook URL"
              rules={[{ required: true }]}
              placeholder="https://..."
            />
          );
        }
        if (type === 'webhook') {
          return (
            <>
              <ProFormText name="url" label="URL" rules={[{ required: true }]} placeholder="https://..." />
              <ProFormText name="token" label="Token(可选)" placeholder="Bearer token" />
            </>
          );
        }
        if (type === 'email') {
          return (
            <>
              <ProFormText name="smtp_host" label="SMTP Host" rules={[{ required: true }]} placeholder="smtp.example.com" />
              <ProFormText name="smtp_port" label="SMTP Port" rules={[{ required: true }]} placeholder="587" />
              <ProFormText name="username" label="用户名" rules={[{ required: true }]} />
              <ProFormText.Password name="password" label="密码" rules={[{ required: true }]} />
              <ProFormSelect
                name="tls_mode"
                label="连接方式"
                initialValue="plain"
                options={[
                  { label: 'Plain', value: 'plain' },
                  { label: 'STARTTLS', value: 'starttls' },
                  { label: 'TLS(465)', value: 'tls' },
                ]}
                width="sm"
              />
              <ProFormText name="from" label="发件人(可选)" placeholder="默认使用用户名" />
              <ProFormText name="subject" label="主题(可选)" placeholder="默认 [Argus Alarm] 状态" />
            </>
          );
        }
        return null;
      }}
    </ProFormDependency>
  );

  const columns: ProColumns[] = [
    {
      title: '名称',
      dataIndex: 'name',
      copyable: true,
      ellipsis: true,
      formItemProps: {
        rules: [{ required: true, message: '此项为必填项' }],
      },
    },
    {
      title: '类型',
      dataIndex: 'type',
      valueType: 'select',
      valueEnum: {
        dingtalk: { text: '钉钉', status: 'Processing' },
        feishu: { text: '飞书', status: 'Processing' },
        wechat: { text: '企业微信', status: 'Processing' },
        slack: { text: 'Slack', status: 'Processing' },
        email: { text: '邮件', status: 'Success' },
        webhook: { text: 'Webhook', status: 'Default' },
      },
    },
    {
      title: '团队',
      dataIndex: 'team_id',
      valueType: 'select',
      request: async () => {
        const res = await getTeams();
        const data = Array.isArray(res) ? res : (res.data || []);
        return data.map((t: any) => ({ label: t.name, value: t.id }));
      },
      render: (_, record) => record.Team?.name || record.team?.name || '-',
    },
    {
      title: '引用规则',
      dataIndex: ['usage', 'count'],
      hideInSearch: true,
      width: 100,
      render: (_, record) => {
        const n = record?.usage?.count || 0;
        return <Tag color={n > 0 ? 'blue' : 'default'}>{n}</Tag>;
      },
    },
    {
      title: '规则列表',
      dataIndex: ['usage', 'rules'],
      hideInSearch: true,
      ellipsis: true,
      render: (_, record) => {
        const rules = record?.usage?.rules || [];
        const text = rules.map((r: any) => r.name).join(', ');
        return (
          <Text ellipsis={{ tooltip: text }} style={{ width: 360 }}>
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
          key="test"
          title="测试发送"
          trigger={<a>测试</a>}
          initialValues={{ format: 'text' }}
          onFinish={async (values) => {
            try {
              await testNotificationChannel(record.id, values);
              message.success('发送成功');
              return true;
            } catch (e) {
              const err: any = (e as any)?.response?.data?.error || (e as any)?.message || '发送失败';
              message.error(String(err));
              return false;
            }
          }}
        >
          <ProFormSelect
            name="format"
            label="格式"
            rules={[{ required: true }]}
            options={[
              { label: 'Text', value: 'text' },
              { label: 'Markdown', value: 'markdown' },
            ]}
          />
          {record.type === 'email' ? (
            <ProFormText name="to" label="收件人(测试用)" rules={[{ required: true }]} placeholder="a@x.com,b@y.com" />
          ) : null}
          <ProFormText
            name="content"
            label="内容"
            rules={[{ required: true }]}
            initialValue="Argus test notification"
          />
        </ModalForm>,
        <ModalForm
          key="edit"
          title="编辑通道"
          trigger={<a>编辑</a>}
          initialValues={parseRecord(record)}
          onFinish={async (values) => {
            try {
              await updateNotificationChannel(record.id, buildSubmit(values));
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
            name="type"
            label="类型"
            rules={[{ required: true }]}
            options={[
              { label: '钉钉', value: 'dingtalk' },
              { label: '飞书', value: 'feishu' },
              { label: '企业微信', value: 'wechat' },
              { label: 'Slack', value: 'slack' },
              { label: '邮件', value: 'email' },
              { label: 'Webhook', value: 'webhook' },
            ]}
          />
          <ProFormSelect
            name="team_id"
            label="团队"
            request={async () => {
              const res = await getTeams();
              const data = Array.isArray(res) ? res : (res.data || []);
              return data.map((t: any) => ({ label: t.name, value: t.id }));
            }}
            fieldProps={{ allowClear: true }}
          />
          {renderConfigFields()}
        </ModalForm>,
        <Popconfirm
          key="delete"
          title="确定要删除吗？"
          onConfirm={async () => {
            try {
              await deleteNotificationChannel(record.id);
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
          const res = await getNotificationChannels(params);
          const data = Array.isArray(res) ? res : (res.data || []);
          return {
            data,
            success: true,
            total: data.length,
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
      headerTitle="通知通道"
      toolBarRender={() => [
        <ModalForm
          key="create"
          title="新建通道"
          trigger={
            <Button type="primary">
              <PlusOutlined />
              新建
            </Button>
          }
          onFinish={async (values) => {
            try {
              await addNotificationChannel(buildSubmit(values));
              message.success('添加成功');
              actionRef.current?.reload();
              return true;
            } catch (e) {
              message.error('添加失败');
              return false;
            }
          }}
          initialValues={{ type: 'dingtalk' }}
        >
          <ProFormText name="name" label="名称" rules={[{ required: true }]} />
          <ProFormSelect
            name="type"
            label="类型"
            rules={[{ required: true }]}
            options={[
              { label: '钉钉', value: 'dingtalk' },
              { label: '飞书', value: 'feishu' },
              { label: '企业微信', value: 'wechat' },
              { label: 'Slack', value: 'slack' },
              { label: '邮件', value: 'email' },
              { label: 'Webhook', value: 'webhook' },
            ]}
          />
          <ProFormSelect
            name="team_id"
            label="团队"
            request={async () => {
              const res = await getTeams();
              const data = Array.isArray(res) ? res : (res.data || []);
              return data.map((t: any) => ({ label: t.name, value: t.id }));
            }}
            fieldProps={{ allowClear: true }}
          />
          {renderConfigFields()}
        </ModalForm>,
      ]}
    />
  );
};

export default NotificationChannelList;
