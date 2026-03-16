import React, { useEffect, useMemo, useState } from 'react';
import { ProLayout, PageContainer } from '@ant-design/pro-components';
import { Outlet, useNavigate, useLocation } from 'react-router-dom';
import { DashboardOutlined, DatabaseOutlined, AlertOutlined, LogoutOutlined, RobotOutlined, UserOutlined, MailOutlined, SafetyOutlined, PlayCircleOutlined, TeamOutlined, HistoryOutlined, NotificationOutlined, ArrowUpOutlined, PauseCircleOutlined, ThunderboltOutlined, StopOutlined, SendOutlined } from '@ant-design/icons';
import { Dropdown } from 'antd';
import { getUserInfo } from '../../services/user';

const Layout: React.FC = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const [me, setMe] = useState<any>(null);

  const handleLogout = () => {
    localStorage.removeItem('token');
    navigate('/login');
  };

  useEffect(() => {
    (async () => {
      try {
        const r: any = await getUserInfo();
        setMe(r);
      } catch (e) {
        setMe(null);
      }
    })();
  }, []);

  const routes = useMemo(() => {
    const isAdmin = String(me?.role || '') === 'admin';
    const base = [
      {
        name: '态势',
        icon: <DashboardOutlined />,
        path: '/dashboard',
        routes: [
          {
            path: '/dashboard',
            name: '态势感知',
          },
          {
            path: '/services',
            name: '服务工作台',
          },
        ],
      },
      {
        icon: <DatabaseOutlined />,
        name: '数据治理',
        path: '/datasources',
        routes: [
          {
            path: '/datasources',
            name: '数据源',
          },
          {
            path: '/datanames',
            name: '数据名',
          },
          {
            path: '/data-query',
            name: '数据查询',
          },
        ],
      },
      {
        icon: <AlertOutlined />,
        name: '告警中心',
        path: '/alert-rules',
        routes: [
          {
            path: '/alert-rules',
            name: '告警规则',
            icon: <AlertOutlined />,
          },
          {
            path: '/alarms',
            name: '告警列表',
            icon: <ThunderboltOutlined />,
          },
          {
            path: '/incidents',
            name: '聚合事件',
            icon: <AlertOutlined />,
          },
          {
            path: '/alert-logs',
            name: '触发记录',
            icon: <HistoryOutlined />,
          },
          {
            path: '/escalations',
            name: '告警升级',
            icon: <ArrowUpOutlined />,
          },
          {
            path: '/routing-rules',
            name: '告警路由',
            icon: <SendOutlined />,
          },
          {
            path: '/silences',
            name: '告警静默',
            icon: <PauseCircleOutlined />,
          },
          {
            path: '/inhibit-rules',
            name: '告警抑制',
            icon: <StopOutlined />,
          },
        ],
      },
      {
        icon: <PlayCircleOutlined />,
        name: '自动化',
        path: '/executions',
        routes: [
          {
            path: '/executions',
            name: '执行记录',
            icon: <PlayCircleOutlined />,
          },
        ],
      },
      {
        icon: <RobotOutlined />,
        name: 'AI',
        path: '/ai-demo',
        routes: [
          {
            path: '/ai-demo',
            name: 'AI 诊断中心',
            icon: <RobotOutlined />,
          },
        ],
      },
    ] as any[];

    if (isAdmin) {
      base.push({
        icon: <SafetyOutlined />,
        name: '系统管理',
        path: '/users',
        routes: [
          {
            path: '/users',
            name: '用户管理',
            icon: <UserOutlined />,
          },
          {
            path: '/teams',
            name: '团队管理',
            icon: <TeamOutlined />,
          },
          {
            path: '/templates',
            name: '消息模板',
            icon: <MailOutlined />,
          },
          {
            path: '/notification-channels',
            name: '通知通道',
            icon: <NotificationOutlined />,
          },
          {
            path: '/security',
            name: '安全设置',
            icon: <SafetyOutlined />,
          },
          {
            path: '/audit-logs',
            name: '操作审计',
            icon: <SafetyOutlined />,
          },
        ],
      });
    }

    return base;
  }, [me]);

  return (
    <div
      id="test-pro-layout"
      style={{
        height: '100vh',
      }}
    >
      <ProLayout
        title="Argus Monitor"
        logo={null}
        layout="mix"
        navTheme="realDark"
        fixSiderbar
        fixedHeader
        siderWidth={208}
        location={{
          pathname: location.pathname,
        }}
        menuItemRender={(item, dom) => {
          if (!item.path) return <span>{dom}</span>;
          return (
            <a
              onClick={() => {
                navigate(item.path || '/');
              }}
            >
              {dom}
            </a>
          );
        }}
        route={{ routes }}
        avatarProps={{
          src: 'https://gw.alipayobjects.com/zos/antfincdn/efFD%24IOql2/weixintupian_20170331104822.jpg',
          size: 'small',
          title: me?.username || '-',
          render: (_props, dom) => {
            return (
              <Dropdown
                menu={{
                  items: [
                    {
                      key: 'logout',
                      icon: <LogoutOutlined />,
                      label: '退出登录',
                      onClick: handleLogout,
                    },
                  ],
                }}
              >
                {dom}
              </Dropdown>
            );
          },
        }}
      >
        <PageContainer
          token={{
            paddingBlockPageContainerContent: 24,
            paddingInlinePageContainerContent: 24,
          }}
          style={{
            minHeight: '100vh',
            background: 'transparent',
          }}
        >
          <Outlet />
        </PageContainer>
      </ProLayout>
    </div>
  );
};

export default Layout;
