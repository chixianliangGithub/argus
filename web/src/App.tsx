import React from 'react';
import { ConfigProvider, theme } from 'antd';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import Layout from './components/Layout';
import Login from './pages/Login';
import Dashboard from './pages/Dashboard';
import DataSource from './pages/DataSource';
import AlertRule from './pages/AlertRule';
import DataName from './pages/DataName';
import UserList from './pages/User';
import TemplateList from './pages/Template';
import TeamList from './pages/Team';
import AuditLogList from './pages/Audit';
import ExecutionList from './pages/Execution';
import AIDemo from './pages/AIDemo';
import AlertLogList from './pages/AlertLog';
import AlarmList from './pages/Alarm';
import DataQuery from './pages/DataQuery';
import NotificationChannelList from './pages/NotificationChannel';
import EscalationList from './pages/Escalation';
import SilenceList from './pages/Silence';
import IncidentList from './pages/Incident';
import IncidentDetail from './pages/Incident/Detail';
import ServiceWorkbench from './pages/Service';
import InhibitRuleList from './pages/Inhibit';
import RoutingRuleList from './pages/RoutingRule';
import Security from './pages/Security';

const PrivateRoute: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const token = localStorage.getItem('token');
  return token ? <>{children}</> : <Navigate to="/login" />;
};

const App: React.FC = () => {
  return (
    <ConfigProvider
      theme={{
        algorithm: theme.darkAlgorithm,
        token: {
          colorPrimary: '#1677ff',
        },
      }}
    >
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route
            path="/"
            element={
              <PrivateRoute>
                <Layout />
              </PrivateRoute>
            }
          >
            <Route index element={<Navigate to="/dashboard" />} />
            <Route path="dashboard" element={<Dashboard />} />
            <Route path="services" element={<ServiceWorkbench />} />
            <Route path="datasources" element={<DataSource />} />
            <Route path="datanames" element={<DataName />} />
            <Route path="data-query" element={<DataQuery />} />
            <Route path="alert-rules" element={<AlertRule />} />
            <Route path="alarms" element={<AlarmList />} />
            <Route path="incidents" element={<IncidentList />} />
            <Route path="incidents/:id" element={<IncidentDetail />} />
            <Route path="alert-logs" element={<AlertLogList />} />
            <Route path="users" element={<UserList />} />
            <Route path="teams" element={<TeamList />} />
            <Route path="templates" element={<TemplateList />} />
            <Route path="notification-channels" element={<NotificationChannelList />} />
            <Route path="escalations" element={<EscalationList />} />
            <Route path="silences" element={<SilenceList />} />
            <Route path="inhibit-rules" element={<InhibitRuleList />} />
            <Route path="routing-rules" element={<RoutingRuleList />} />
            <Route path="security" element={<Security />} />
            <Route path="audit-logs" element={<AuditLogList />} />
            <Route path="executions" element={<ExecutionList />} />
            <Route path="ai-demo" element={<AIDemo />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </ConfigProvider>
  );
};

export default App;
