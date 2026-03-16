import React, { useRef, useState } from 'react';
import { ProTable, type ProColumns, type ActionType } from '@ant-design/pro-components';
import { Button, message, Popconfirm } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { getDataNames, addDataName, updateDataName, deleteDataName } from '../../services/dataname';
import { getDataSources } from '../../services/datasource';
import { ModalForm, ProFormText, ProFormSelect, ProFormTextArea } from '@ant-design/pro-components';

const DataName: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [dataSources, setDataSources] = useState<any[]>([]);

  // Load DataSources for selection
  const loadDataSources = async () => {
    try {
      const res = await getDataSources();
      // Handle array vs object response
      const data = Array.isArray(res) ? res : (res.data || []);
      setDataSources(data);
    } catch (error) {
      console.error(error);
      message.error('加载数据源失败');
    }
  };

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
      title: '数据源',
      dataIndex: 'data_source_id',
      valueType: 'select',
      render: (_, record) => record.data_source?.name || '-',
      fieldProps: {
        options: dataSources.map((ds) => ({ label: ds.name, value: ds.id })),
      },
      formItemProps: {
        rules: [{ required: true, message: '此项为必填项' }],
      },
    },
    {
      title: '描述',
      dataIndex: 'description',
      hideInSearch: true,
    },
    {
      title: '操作',
      valueType: 'option',
      render: (_text, record, _, _action) => [
        <ModalForm
          key="edit"
          title="编辑数据名称"
          trigger={<a>编辑</a>}
          onFinish={async (values) => {
            try {
              await updateDataName(record.id, values);
              message.success('更新成功');
              actionRef.current?.reload();
              return true;
            } catch (error) {
              message.error('更新失败');
              return false;
            }
          }}
          initialValues={{
            ...record,
            data_source_id: record.data_source_id, // Handle casing mismatch if any
          }}
          modalProps={{
            afterOpenChange: (open: boolean) => {
              if (open) loadDataSources();
            }
          }}
        >
          <ProFormText name="name" label="名称" rules={[{ required: true }]} />
          <ProFormSelect
            name="data_source_id"
            label="数据源"
            options={dataSources.map((ds) => ({ label: ds.name, value: ds.id }))}
            rules={[{ required: true }]}
          />
          <ProFormText
            name="time_field"
            label="时间字段"
            tooltip="用于告警触发时间/时间窗口的字段名，例如 ts、create_time、@timestamp"
            placeholder="ts"
          />
          <ProFormTextArea name="description" label="描述" />
        </ModalForm>,
        <Popconfirm
          key="delete"
          title="确定要删除吗？"
          onConfirm={async () => {
            try {
              await deleteDataName(record.id);
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
          // Load datasources first to ensure mapping works
          await loadDataSources();
          const res = await getDataNames(params);
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
      headerTitle="数据名称列表"
      toolBarRender={() => [
        <ModalForm
          key="create"
          title="新建数据名称"
          trigger={
            <Button type="primary">
              <PlusOutlined />
              新建
            </Button>
          }
          onFinish={async (values) => {
            try {
              await addDataName(values);
              message.success('添加成功');
              actionRef.current?.reload();
              return true;
            } catch (error) {
              message.error('添加失败');
              return false;
            }
          }}
          modalProps={{
            afterOpenChange: (open: boolean) => {
              if (open) loadDataSources();
            }
          }}
        >
          <ProFormText name="name" label="名称" rules={[{ required: true }]} />
          <ProFormSelect
            name="data_source_id"
            label="数据源"
            options={dataSources.map((ds) => ({ label: ds.name, value: ds.id }))}
            rules={[{ required: true }]}
          />
          <ProFormText
            name="time_field"
            label="时间字段"
            tooltip="用于告警触发时间/时间窗口的字段名，例如 ts、create_time、@timestamp"
            placeholder="ts"
          />
          <ProFormTextArea name="description" label="描述" />
        </ModalForm>,
      ]}
    />
  );
};

export default DataName;
