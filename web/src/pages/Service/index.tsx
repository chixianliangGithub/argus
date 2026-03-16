import React, { useEffect, useMemo, useState } from 'react';
import { Button, Card, Col, Drawer, Input, Row, Segmented, Space, Spin, Tag, Typography, message } from 'antd';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { getServiceSummary, getServicesOverview } from '../../services/service';
import EChartsView from '../../components/EChartsView';
import { matchRunbooks } from '../../services/runbook';

const { Text } = Typography;

const statusMeta = (s: string) => {
  if (s === 'down') return { text: '故障', color: '#a8071a' };
  if (s === 'degraded') return { text: '降级', color: '#faad14' };
  return { text: '健康', color: '#52c41a' };
};

const ServiceWorkbench: React.FC = () => {
  const navigate = useNavigate();
  const [sp, setSp] = useSearchParams();

  const [window, setWindow] = useState<'6h' | '24h' | '168h'>('24h');
  const [q, setQ] = useState(sp.get('q') || '');
  const [loading, setLoading] = useState(false);
  const [rows, setRows] = useState<any[]>([]);
  const [changes, setChanges] = useState<any[]>([]);

  const selected = sp.get('selected') || '';
  const [drawerOpen, setDrawerOpen] = useState(!!selected);
  const [drawerLoading, setDrawerLoading] = useState(false);
  const [drawerData, setDrawerData] = useState<any>(null);
  const [rbLoading, setRbLoading] = useState(false);
  const [rbList, setRbList] = useState<any[]>([]);

  const reload = async () => {
    setLoading(true);
    try {
      const res: any = await getServicesOverview({ window, q: q.trim() || undefined });
      setRows(Array.isArray(res?.services) ? res.services : []);
      setChanges(Array.isArray(res?.changes) ? res.changes : []);
    } catch (e) {
      setRows([]);
      setChanges([]);
    } finally {
      setLoading(false);
    }
  };

  const openService = async (service: string) => {
    if (!service) return;
    setDrawerOpen(true);
    setDrawerLoading(true);
    setDrawerData(null);
    setRbList([]);
    try {
      const res: any = await getServiceSummary(service, { window });
      setDrawerData(res);
    } catch (e: any) {
      setDrawerData(null);
      const msg = e?.response?.data?.error || e?.message || '加载失败';
      message.error(msg);
    } finally {
      setDrawerLoading(false);
    }
  };

  useEffect(() => {
    reload();
  }, [window]);

  useEffect(() => {
    if (selected) {
      openService(selected);
    } else {
      setDrawerOpen(false);
      setDrawerData(null);
    }
  }, [selected]);

  const cards = useMemo(() => {
    return rows.map((r) => {
      const m = statusMeta(r.status);
      const buckets: string[] = Array.isArray(r.buckets) ? r.buckets : [];
      const trend: number[] = Array.isArray(r.alarm_trend) ? r.alarm_trend : [];
      const x = buckets.map((b) => String(b).slice(11, 13));
      const option = {
        grid: { left: 6, right: 6, top: 8, bottom: 0, containLabel: false },
        xAxis: { type: 'category', show: false, data: x },
        yAxis: { type: 'value', show: false },
        series: [
          {
            type: 'line',
            smooth: true,
            symbol: 'none',
            data: trend,
            lineStyle: { width: 2, color: m.color },
            areaStyle: { opacity: 0.12, color: m.color },
          },
        ],
        tooltip: { trigger: 'axis' },
      };

      return (
        <Col key={`${r.service}__${r.app || ''}`} xs={24} sm={12} lg={8} xl={6}>
          <Card
            size="small"
            title={
              <Space size={8}>
                <Tag color={m.color}>{m.text}</Tag>
                <Text ellipsis style={{ width: 160 }}>
                  {r.service}
                </Text>
                {r.app ? <Tag color="default">{r.app}</Tag> : null}
              </Space>
            }
            extra={<Tag color="default">{r.health_score}</Tag>}
            actions={[
              <Button
                key="alarms"
                type="link"
                onClick={() => navigate(`/alarms?status=firing&service=${encodeURIComponent(r.service)}`)}
              >
                告警
              </Button>,
              <Button key="incidents" type="link" onClick={() => navigate(`/incidents?service=${encodeURIComponent(r.service)}`)}>
                事件
              </Button>,
              <Button key="ai" type="link" onClick={() => navigate(`/ai-demo?service=${encodeURIComponent(r.service)}`)}>
                AI
              </Button>,
              <Button
                key="detail"
                type="link"
                onClick={() => {
                  const next = new URLSearchParams(sp);
                  next.set('selected', r.service);
                  setSp(next);
                }}
              >
                详情
              </Button>,
            ]}
          >
            <Row gutter={[8, 8]}>
              <Col span={8}>
                <div style={{ color: '#999' }}>Firing</div>
                <div style={{ fontSize: 18 }}>{r.firing || 0}</div>
              </Col>
              <Col span={8}>
                <div style={{ color: '#999' }}>事件Open</div>
                <div style={{ fontSize: 18 }}>{r.open_incidents || 0}</div>
              </Col>
              <Col span={8}>
                <div style={{ color: '#999' }}>待认领</div>
                <div style={{ fontSize: 18 }}>{r.unclaimed_firing || 0}</div>
              </Col>
              <Col span={24}>
                <EChartsView option={option} height={86} />
              </Col>
            </Row>
          </Card>
        </Col>
      );
    });
  }, [rows, sp, setSp, navigate]);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      <Card
        size="small"
        title="服务工作台"
        extra={
          <Space>
            <Input.Search
              allowClear
              value={q}
              placeholder="搜索 service/app"
              onChange={(e) => setQ(e.target.value)}
              onSearch={() => reload()}
              style={{ width: 260 }}
            />
            <Segmented
              value={window}
              onChange={(v) => setWindow(v as any)}
              options={[
                { label: '6h', value: '6h' },
                { label: '24h', value: '24h' },
                { label: '7d', value: '168h' },
              ]}
            />
            <Button onClick={reload}>刷新</Button>
          </Space>
        }
      >
        {loading ? (
          <div style={{ padding: 24 }}>
            <Spin />
          </div>
        ) : (
          <Row gutter={[12, 12]}>{cards}</Row>
        )}
      </Card>

      <Card size="small" title="最近变更" extra={<Tag color="default">来自审计日志</Tag>}>
        {changes.length === 0 ? (
          <div style={{ color: '#999' }}>暂无数据</div>
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            {changes.slice(0, 10).map((x: any) => (
              <div key={x.id} style={{ display: 'flex', gap: 8 }}>
                <Tag color="default">{x.action}</Tag>
                <Tag color="blue">{x.resource}</Tag>
                <Text ellipsis style={{ flex: 1 }}>
                  {x.details || '-'}
                </Text>
                <span style={{ color: '#999' }}>{x.created_at}</span>
              </div>
            ))}
          </div>
        )}
      </Card>

      <Drawer
        title={drawerData?.service ? `服务详情 - ${drawerData.service}` : '服务详情'}
        open={drawerOpen}
        width={980}
        onClose={() => {
          const next = new URLSearchParams(sp);
          next.delete('selected');
          setSp(next);
          setDrawerOpen(false);
        }}
      >
        {drawerLoading ? (
          <Spin />
        ) : drawerData ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            <Card size="small" title="Top 规则">
              {Array.isArray(drawerData?.top_rules) && drawerData.top_rules.length > 0 ? (
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
                  {drawerData.top_rules.map((r: any) => (
                    <Tag key={r.rule_name} color="red">
                      {r.rule_name} ({r.cnt})
                    </Tag>
                  ))}
                </div>
              ) : (
                <div style={{ color: '#999' }}>暂无数据</div>
              )}
            </Card>

            <Card
              size="small"
              title="推荐 Runbook"
              extra={
                <Button
                  type="primary"
                  loading={rbLoading}
                  onClick={async () => {
                    setRbLoading(true);
                    try {
                      const query = `service=${drawerData.service}\n` + (drawerData?.top_rules || []).map((x: any) => x.rule_name).join('\n');
                      const res: any = await matchRunbooks({ query });
                      const arr = Array.isArray(res?.data) ? res.data : res?.data || res?.matches || [];
                      setRbList(Array.isArray(arr) ? arr : []);
                    } catch (e) {
                      setRbList([]);
                    } finally {
                      setRbLoading(false);
                    }
                  }}
                >
                  推荐
                </Button>
              }
            >
              {rbList.length === 0 ? (
                <div style={{ color: '#999' }}>暂无数据</div>
              ) : (
                <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                  {rbList.slice(0, 5).map((rb: any) => (
                    <Card key={rb.id} size="small" title={rb.name} extra={<Tag color="default">{rb.trigger_type}</Tag>}>
                      <div style={{ color: '#999', whiteSpace: 'pre-wrap' }}>{rb.description || '-'}</div>
                    </Card>
                  ))}
                </div>
              )}
            </Card>
          </div>
        ) : (
          <div style={{ color: '#999' }}>暂无数据</div>
        )}
      </Drawer>
    </div>
  );
};

export default ServiceWorkbench;

