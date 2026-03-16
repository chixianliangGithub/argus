import React, { useRef } from 'react';
import { ProTable, type ProColumns, type ActionType } from '@ant-design/pro-components';
import { Button, message, Popconfirm } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { getDataSources, addDataSource, updateDataSource, deleteDataSource } from '../../services/datasource';
import { ModalForm, ProFormText, ProFormSelect, ProFormTextArea, ProFormDependency, ProFormSwitch } from '@ant-design/pro-components';

const DataSource: React.FC = () => {
  const actionRef = useRef<ActionType>();

  const buildInitialValues = (record?: any) => {
    if (!record) {
      return { es_version: '8.x', es_https: false, es_insecure_skip_verify: false };
    }
    if (record.type !== 'elasticsearch') return record;
    let cfg: any = {};
    try {
      cfg = record.config ? JSON.parse(record.config) : {};
    } catch (e) {
      cfg = {};
    }
    const url: string = record.url || '';
    const httpsFromUrl = url.startsWith('https://');
    const stripped = url.replace(/^https?:\/\//, '');
    return {
      ...record,
      url: stripped,
      es_version: cfg?.version || '8.x',
      es_https: cfg?.tls?.enabled ?? httpsFromUrl,
      es_insecure_skip_verify: !!cfg?.tls?.insecure_skip_verify,
      es_ca_pem: cfg?.tls?.ca_pem || '',
      es_api_key: cfg?.api_key || '',
    };
  };

  const buildSubmitValues = (values: any) => {
    const v: any = { ...values };
    if (v.type !== 'elasticsearch') return v;
    const https = !!v.es_https;
    let addr = String(v.url || '').trim();
    if (addr && !addr.startsWith('http://') && !addr.startsWith('https://')) {
      addr = (https ? 'https://' : 'http://') + addr;
    }
    v.url = addr;
    const cfg: any = {
      version: v.es_version || '8.x',
      tls: {
        enabled: https,
        insecure_skip_verify: !!v.es_insecure_skip_verify,
        ca_pem: v.es_ca_pem || '',
      },
    };
    if (String(v.es_api_key || '').trim() !== '') {
      cfg.api_key = String(v.es_api_key || '').trim();
    }
    v.config = JSON.stringify(cfg);
    delete v.es_version;
    delete v.es_https;
    delete v.es_insecure_skip_verify;
    delete v.es_ca_pem;
    delete v.es_api_key;
    return v;
  };

  const renderFormItems = () => (
    <>
      <ProFormText name="name" label="名称" rules={[{ required: true }]} />
      <ProFormSelect
        name="type"
        label="类型"
        options={[
          { label: 'Prometheus', value: 'prometheus' },
          { label: 'MySQL', value: 'mysql' },
          { label: 'Elasticsearch', value: 'elasticsearch' },
          { label: 'SkyWalking', value: 'skywalking' },
          { label: 'InfluxDB', value: 'influxdb' },
          { label: 'ClickHouse', value: 'clickhouse' },
          { label: 'SqlServer', value: 'sqlserver' },
          { label: 'IoTDB', value: 'iotdb' },
        ]}
        rules={[{ required: true }]}
      />
      
      <ProFormDependency name={['type']}>
        {({ type }) => {
          let urlPlaceholder = '请输入URL';
          let urlLabel = 'URL';
          let showAuth = true;
          let showDB = true;
          let dbLabel = '数据库名称';
          let dbPlaceholder = type === 'mysql' ? 'dbname' : '请输入数据库名称';
          
          switch (type) {
            case 'prometheus':
              urlPlaceholder = 'http://localhost:9090';
              showDB = false;
              break;
            case 'mysql':
              urlPlaceholder = '127.0.0.1:3306';
              urlLabel = '主机地址';
              dbPlaceholder = 'dbname';
              break;
            case 'elasticsearch':
              urlPlaceholder = 'localhost:9200';
              urlLabel = '服务地址';
              showDB = true;
              dbLabel = '索引/模式';
              dbPlaceholder = 'logs-*';
              break;
            case 'influxdb':
              urlPlaceholder = 'http://localhost:8086';
              break;
            case 'clickhouse':
              urlPlaceholder = 'tcp://127.0.0.1:9000';
              break;
            case 'sqlserver':
              urlPlaceholder = 'localhost:1433';
              urlLabel = '主机地址';
              break;
            case 'iotdb':
              urlPlaceholder = '127.0.0.1:6667';
              urlLabel = '主机地址';
              break;
            case 'skywalking':
              urlPlaceholder = 'http://localhost:12800/graphql';
              showDB = false;
              showAuth = false;
              break;
          }

          return (
            <>
              <ProFormText 
                name="url" 
                label={urlLabel}
                rules={[{ required: true }]}
                placeholder={urlPlaceholder}
              />
              {type === 'elasticsearch' ? (
                <>
                  <ProFormSelect
                    name="es_version"
                    label="版本"
                    rules={[{ required: true }]}
                    options={[
                      { label: '7.x', value: '7.x' },
                      { label: '8.x', value: '8.x' },
                    ]}
                    initialValue="8.x"
                    width="sm"
                  />
                  <ProFormSwitch name="es_https" label="是否HTTPS" initialValue={false} />
                  <ProFormText name="es_api_key" label="API Key" />
                  <ProFormSwitch name="es_insecure_skip_verify" label="跳过证书校验" initialValue={false} />
                  <ProFormTextArea name="es_ca_pem" label="SSL证书(PEM)" fieldProps={{ rows: 4 }} />
                </>
              ) : null}
              {showAuth && (
                <>
                  <ProFormText name="username" label="用户名" />
                  <ProFormText.Password name="password" label="密码" />
                </>
              )}
              {showDB && (
                <ProFormText 
                  name="database" 
                  label={dbLabel} 
                  placeholder={dbPlaceholder}
                />
              )}
            </>
          );
        }}
      </ProFormDependency>
      
      <ProFormTextArea name="description" label="描述" />
    </>
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
        prometheus: { text: 'Prometheus', status: 'Success' },
        mysql: { text: 'MySQL', status: 'Warning' },
        elasticsearch: { text: 'Elasticsearch', status: 'Processing' },
        skywalking: { text: 'SkyWalking', status: 'Default' },
        influxdb: { text: 'InfluxDB', status: 'Default' },
        clickhouse: { text: 'ClickHouse', status: 'Default' },
        sqlserver: { text: 'SqlServer', status: 'Default' },
        iotdb: { text: 'IoTDB', status: 'Default' },
      },
    },
    {
      title: 'URL',
      dataIndex: 'url',
      hideInSearch: true,
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
          title="编辑数据源"
          trigger={<a>编辑</a>}
          onFinish={async (values) => {
            try {
              await updateDataSource(record.id, buildSubmitValues(values));
              message.success('更新成功');
              actionRef.current?.reload();
              return true;
            } catch (error) {
              message.error('更新失败');
              return false;
            }
          }}
          initialValues={buildInitialValues(record)}
        >
          {renderFormItems()}
        </ModalForm>,
        <Popconfirm
          key="delete"
          title="确定要删除吗？"
          onConfirm={async () => {
            try {
              await deleteDataSource(record.id);
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
          const res = await getDataSources(params);
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
      headerTitle="数据源列表"
      toolBarRender={() => [
        <ModalForm
          key="create"
          title="新建数据源"
          initialValues={buildInitialValues()}
          trigger={
            <Button type="primary">
              <PlusOutlined />
              新建
            </Button>
          }
          onFinish={async (values) => {
            try {
              const res = await addDataSource(buildSubmitValues(values));
              if (res) {
                message.success('添加成功');
                actionRef.current?.reload();
                return true;
              }
              return false;
            } catch (error) {
              message.error('添加失败');
              return false;
            }
          }}
        >
          {renderFormItems()}
        </ModalForm>,
      ]}
    />
  );
};

export default DataSource;
