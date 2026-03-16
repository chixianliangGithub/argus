import React, { useEffect, useMemo, useState } from 'react';
import { Button, Card, message, Table } from 'antd';
import { ProForm, ProFormSelect, ProFormTextArea } from '@ant-design/pro-components';
import { useNavigate } from 'react-router-dom';
import { getDataNames } from '../../services/dataname';
import { previewAlertRule } from '../../services/alertRule';

const DataQuery: React.FC = () => {
  const navigate = useNavigate();
  const [dataNames, setDataNames] = useState<any[]>([]);
  const [previewData, setPreviewData] = useState<any[]>([]);
  const [previewColumns, setPreviewColumns] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [lastQuery, setLastQuery] = useState<{ data_name_id: number; query: string } | null>(null);

  useEffect(() => {
    (async () => {
      try {
        const dnRes = await getDataNames();
        setDataNames(Array.isArray(dnRes) ? dnRes : (dnRes.data || []));
      } catch (e) {}
    })();
  }, []);

  const columns = useMemo(() => {
    if (previewData.length === 0) return [];
    const first = previewData[0];
    if (!first || typeof first !== 'object') return [];
    return Object.keys(first).map((k) => ({ title: k, dataIndex: k, key: k }));
  }, [previewData]);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <Card title="数据查询" size="small">
        <ProForm
          submitter={{
            searchConfig: { submitText: '查询', resetText: '重置' },
          }}
          onFinish={async (values) => {
            if (!values.data_name_id || !values.query) {
              message.error('请先选择数据名并输入查询语句');
              return false;
            }
            setLoading(true);
            try {
              const res = await previewAlertRule({ data_name_id: values.data_name_id, query: values.query });
              const data = res.data || [];
              let tableData: any[] = [];
              if (Array.isArray(data)) tableData = data;
              else if (typeof data === 'object' && data !== null) tableData = [data];
              setPreviewData(tableData);
              if (tableData.length > 0 && typeof tableData[0] === 'object' && tableData[0] !== null) {
                setPreviewColumns(Object.keys(tableData[0]));
              } else {
                setPreviewColumns([]);
              }
              setLastQuery({ data_name_id: values.data_name_id, query: values.query });
              message.success('查询成功');
              return true;
            } catch (e) {
              message.error('查询失败');
              return false;
            } finally {
              setLoading(false);
            }
          }}
        >
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
            fieldProps={{ rows: 6, style: { fontFamily: 'monospace' } }}
          />
        </ProForm>
      </Card>

      <Card
        title="查询结果"
        size="small"
        extra={
          <Button
            disabled={!lastQuery}
            onClick={() => {
              if (!lastQuery) return;
              navigate('/alert-rules', {
                state: {
                  prefill: {
                    name: `新告警规则-${new Date().toISOString().slice(11, 19).replace(/:/g, '')}`,
                    data_name_id: lastQuery.data_name_id,
                    query: lastQuery.query,
                    is_enabled: false,
                    value_mode: 'row_count',
                    row_pick: 'first',
                    condition: '>',
                    threshold: 0,
                  },
                },
              });
            }}
          >
            生成告警规则
          </Button>
        }
      >
        <Table
          dataSource={previewData}
          columns={columns}
          loading={loading}
          scroll={{ x: true }}
          pagination={{ pageSize: 20 }}
          rowKey={(_, idx) => String(idx)}
          size="small"
        />
        {previewColumns.length > 0 && (
          <div style={{ marginTop: 8, color: '#888' }}>
            字段：{previewColumns.join(', ')}
          </div>
        )}
      </Card>
    </div>
  );
};

export default DataQuery;
