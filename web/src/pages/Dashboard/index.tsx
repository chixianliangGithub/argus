import React, { useEffect, useState } from 'react';
import { Row, Col, Table, Tag, Card, Statistic, List } from 'antd';
import { useNavigate } from 'react-router-dom';
import Dashboard3D from '../../components/Dashboard3D';
import TopologyView from '../../components/TopologyView';
import { getIncidents } from '../../services/incident';
import { getAlarms } from '../../services/alarm';

const Dashboard: React.FC = () => {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);
  const [incidents, setIncidents] = useState<any[]>([]);
  const [kpiLoading, setKpiLoading] = useState(false);
  const [kpi, setKpi] = useState<{ openIncidents: number; ackedIncidents: number; resolvedIncidents: number; firingAlarms: number; unclaimedAlarms: number }>({
    openIncidents: 0,
    ackedIncidents: 0,
    resolvedIncidents: 0,
    firingAlarms: 0,
    unclaimedAlarms: 0,
  });
  const [topRules, setTopRules] = useState<Array<{ name: string; count: number }>>([]);

  useEffect(() => {
    (async () => {
      setLoading(true);
      try {
        const res = await getIncidents({ status: 'open', current: 1, pageSize: 10 });
        const data = Array.isArray(res?.data) ? res.data : res?.data || [];
        setIncidents(data);
      } catch (e) {
        setIncidents([]);
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  useEffect(() => {
    (async () => {
      setKpiLoading(true);
      try {
        const [openInc, ackedInc, resolvedInc, firingAlarm, unclaimedAlarm, topAlarmRes] = (await Promise.all([
          getIncidents({ status: 'open', current: 1, pageSize: 1 }),
          getIncidents({ status: 'acked', current: 1, pageSize: 1 }),
          getIncidents({ status: 'resolved', current: 1, pageSize: 1 }),
          getAlarms({ status: 'firing', current: 1, pageSize: 1 }),
          getAlarms({ status: 'firing', claimed: 0, current: 1, pageSize: 1 }),
          getAlarms({ status: 'firing', current: 1, pageSize: 50 }),
        ])) as any;

        setKpi({
          openIncidents: Number(openInc?.total || 0),
          ackedIncidents: Number(ackedInc?.total || 0),
          resolvedIncidents: Number(resolvedInc?.total || 0),
          firingAlarms: Number(firingAlarm?.total || 0),
          unclaimedAlarms: Number(unclaimedAlarm?.total || 0),
        });

        const rows = Array.isArray(topAlarmRes?.data) ? topAlarmRes.data : topAlarmRes?.data || [];
        const counter: Record<string, number> = {};
        for (const r of rows) {
          const key = String(r.rule_name || r.alert_rule_id || 'unknown');
          counter[key] = (counter[key] || 0) + 1;
        }
        const top = Object.entries(counter)
          .map(([name, count]) => ({ name, count }))
          .sort((a, b) => b.count - a.count)
          .slice(0, 5);
        setTopRules(top);
      } catch (e) {
        setKpi({
          openIncidents: 0,
          ackedIncidents: 0,
          resolvedIncidents: 0,
          firingAlarms: 0,
          unclaimedAlarms: 0,
        });
        setTopRules([]);
      } finally {
        setKpiLoading(false);
      }
    })();
  }, []);

  return (
    <div className="space-y-6">
      <h2 className="text-2xl font-bold text-cyber-primary mb-4 neon-text">态势感知仪表盘</h2>

      <Row gutter={[24, 24]}>
        <Col span={24} lg={4}>
          <Card size="small" loading={kpiLoading}>
            <Statistic title="事件 Open" value={kpi.openIncidents} />
          </Card>
        </Col>
        <Col span={24} lg={4}>
          <Card size="small" loading={kpiLoading}>
            <Statistic title="事件 Acked" value={kpi.ackedIncidents} />
          </Card>
        </Col>
        <Col span={24} lg={4}>
          <Card size="small" loading={kpiLoading}>
            <Statistic title="事件 Resolved" value={kpi.resolvedIncidents} />
          </Card>
        </Col>
        <Col span={24} lg={4}>
          <Card size="small" loading={kpiLoading}>
            <Statistic title="告警 Firing" value={kpi.firingAlarms} />
          </Card>
        </Col>
        <Col span={24} lg={4}>
          <Card size="small" loading={kpiLoading}>
            <Statistic title="待认领告警" value={kpi.unclaimedAlarms} />
          </Card>
        </Col>
        <Col span={24} lg={4}>
          <Card size="small" loading={kpiLoading} title="Top 规则">
            <List
              size="small"
              dataSource={topRules}
              renderItem={(item) => (
                <List.Item>
                  <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{item.name}</span>
                  <Tag color="red">{item.count}</Tag>
                </List.Item>
              )}
            />
          </Card>
        </Col>
      </Row>
      
      <Row gutter={[24, 24]}>
        <Col span={24} lg={12}>
          <Dashboard3D />
        </Col>
        <Col span={24} lg={12}>
          <TopologyView />
        </Col>
      </Row>

      <Row gutter={[24, 24]}>
        <Col span={24}>
          <div className="glass-panel p-6 rounded-lg">
             <h3 className="text-xl text-cyber-secondary mb-4">实时事件列表（Open）</h3>
             <Table
               rowKey="id"
               size="small"
               loading={loading}
               pagination={false}
               dataSource={incidents}
               columns={[
                 {
                   title: '标题',
                   dataIndex: 'title',
                   render: (_: any, r: any) => <a onClick={() => navigate(`/incidents/${r.id}`)}>{r.title || `#${r.id}`}</a>,
                 },
                 {
                   title: '级别',
                   dataIndex: 'severity',
                   width: 110,
                   render: (_: any, r: any) => {
                     const s = r.severity;
                     const color = s === 'critical' ? 'red' : s === 'warning' ? 'gold' : 'blue';
                     return <Tag color={color}>{s || '-'}</Tag>;
                   },
                 },
                 { title: '团队', dataIndex: 'team_id', width: 90 },
                 { title: '最近活动', dataIndex: 'last_activity_at', width: 190 },
               ]}
             />
          </div>
        </Col>
      </Row>
    </div>
  );
};

export default Dashboard;
