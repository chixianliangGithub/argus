import React, { useRef } from 'react';
import { ProTable, type ProColumns, type ActionType } from '@ant-design/pro-components';
import { Button, Card, message, Popconfirm, Tag } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { getTemplates, addTemplate, updateTemplate, deleteTemplate } from '../../services/template';
import { ModalForm, ProFormText, ProFormSelect, ProFormTextArea, ProFormSwitch } from '@ant-design/pro-components';

const TemplateList: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const demo = [
    '规则：{{ .Rule.Name }}',
    '级别：{{ .Rule.Level }}',
    '触发时间：{{ .TriggerAt }}',
    '发生时间(来自样例行 time_field)：{{ .EventAt }}',
    '发送时间：{{ .SendAt }}',
    'trace_id：{{ default "-" (index .Row "trace_id") }}',
    'log_message：{{ truncate (default "" (index .Row "log_message")) 200 }}',
    '内容：',
    '{{ .Content }}',
  ].join('\n');

  const columns: ProColumns[] = [
    {
      title: '模板名称',
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
        general: { text: '通用', status: 'Default' },
        email: { text: '邮件', status: 'Success' },
        dingtalk: { text: '钉钉', status: 'Processing' },
        feishu: { text: '飞书', status: 'Processing' },
        wechat: { text: '企业微信', status: 'Processing' },
        slack: { text: 'Slack', status: 'Processing' },
        webhook: { text: 'Webhook', status: 'Default' },
      },
    },
    {
      title: '格式',
      dataIndex: 'format',
      valueType: 'select',
      valueEnum: {
        text: { text: 'Text', status: 'Default' },
        markdown: { text: 'Markdown', status: 'Processing' },
      },
    },
    {
      title: '默认',
      dataIndex: 'is_default',
      render: (_, record) => (
        <Tag color={record.is_default ? 'blue' : 'default'}>
          {record.is_default ? '是' : '否'}
        </Tag>
      ),
    },
    {
      title: '操作',
      valueType: 'option',
      render: (_text, record, _, _action) => [
        <ModalForm
          key="edit"
          title="编辑模板"
          trigger={<a>编辑</a>}
          onFinish={async (values) => {
            try {
              await updateTemplate(record.id, values);
              message.success('更新成功');
              actionRef.current?.reload();
              return true;
            } catch (error) {
              message.error('更新失败');
              return false;
            }
          }}
          initialValues={record}
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
            initialValue="general"
          />
          <ProFormSelect
            name="format"
            label="格式"
            initialValue="text"
            options={[
              { label: 'Text', value: 'text' },
              { label: 'Markdown', value: 'markdown' },
            ]}
            rules={[{ required: true }]}
          />
          <ProFormTextArea 
            name="content" 
            label="模板内容 (支持 Go Template)" 
            rules={[{ required: true }]} 
            fieldProps={{ rows: 10 }}
            placeholder="{{.Content}}"
          />
          <Card size="small" title="示例" style={{ marginTop: 12 }}>
            <pre style={{ margin: 0, whiteSpace: 'pre-wrap' }}>{demo}</pre>
            <div style={{ marginTop: 8 }}>
              <Button
                onClick={async () => {
                  try {
                    await navigator.clipboard.writeText(demo);
                    message.success('已复制示例到剪贴板');
                  } catch (e) {
                    message.error('复制失败，请手动复制');
                  }
                }}
              >
                复制示例
              </Button>
            </div>
          </Card>
          <ProFormSwitch name="is_default" label="设为默认" />
        </ModalForm>,
        <Popconfirm
          key="delete"
          title="确定要删除吗？"
          onConfirm={async () => {
            try {
              await deleteTemplate(record.id);
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

  return (
    <ProTable
      columns={columns}
      actionRef={actionRef}
      cardBordered
      request={async (params) => {
        try {
          const res = await getTemplates(params);
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
      headerTitle="消息模板列表"
      toolBarRender={() => [
        <ModalForm
          key="create"
          title="新建模板"
          trigger={
            <Button type="primary">
              <PlusOutlined />
              新建
            </Button>
          }
          onFinish={async (values) => {
            try {
              await addTemplate(values);
              message.success('添加成功');
              actionRef.current?.reload();
              return true;
            } catch (error) {
              message.error('添加失败');
              return false;
            }
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
            initialValue="general"
          />
          <ProFormSelect
            name="format"
            label="格式"
            initialValue="text"
            options={[
              { label: 'Text', value: 'text' },
              { label: 'Markdown', value: 'markdown' },
            ]}
            rules={[{ required: true }]}
          />
          <ProFormTextArea 
            name="content" 
            label="模板内容 (支持 Go Template)" 
            rules={[{ required: true }]} 
            fieldProps={{ rows: 10 }}
            placeholder="{{.Content}}"
          />
          <Card size="small" title="示例" style={{ marginTop: 12 }}>
            <pre style={{ margin: 0, whiteSpace: 'pre-wrap' }}>{demo}</pre>
            <div style={{ marginTop: 8 }}>
              <Button
                onClick={async () => {
                  try {
                    await navigator.clipboard.writeText(demo);
                    message.success('已复制示例到剪贴板');
                  } catch (e) {
                    message.error('复制失败，请手动复制');
                  }
                }}
              >
                复制示例
              </Button>
            </div>
          </Card>
          <ProFormSwitch name="is_default" label="设为默认" />
        </ModalForm>,
      ]}
    />
  );
};

export default TemplateList;
