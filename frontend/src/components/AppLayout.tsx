import React, { useState } from 'react';
import { Layout, Menu, Avatar, Dropdown, Typography } from 'antd';
import {
  ProjectOutlined, BookOutlined, UserOutlined,
  HistoryOutlined, LogoutOutlined, MenuFoldOutlined, MenuUnfoldOutlined,
} from '@ant-design/icons';
import { useNavigate, useLocation, Outlet } from 'react-router-dom';
import { clearAuth, currentUser, hasRole } from '../store/auth';

const { Header, Sider, Content } = Layout;
const { Text } = Typography;

const IBCON_COLOR = '#1a3a6b';

export default function AppLayout() {
  const navigate = useNavigate();
  const location = useLocation();
  const user = currentUser();
  const [collapsed, setCollapsed] = useState(false);

  function logout() {
    clearAuth();
    navigate('/login');
  }

  const menuItems = [
    {
      key: '/projects',
      icon: <ProjectOutlined />,
      label: 'Проекты',
    },
    {
      key: '/references',
      icon: <BookOutlined />,
      label: 'Справочники',
    },
    ...(hasRole('GE') ? [{
      key: '/users',
      icon: <UserOutlined />,
      label: 'Пользователи',
    }] : []),
    ...(hasRole('GE', 'EP', 'IP') ? [{
      key: '/audit',
      icon: <HistoryOutlined />,
      label: 'История изменений',
    }] : []),
  ];

  const selectedKey = '/' + location.pathname.split('/')[1];

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider
        collapsible
        collapsed={collapsed}
        onCollapse={setCollapsed}
        theme="dark"
        style={{ background: IBCON_COLOR }}
        trigger={null}
      >
        <div style={{
          height: 64,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          color: 'white',
          fontWeight: 700,
          fontSize: collapsed ? 14 : 16,
          letterSpacing: 1,
          padding: '0 16px',
        }}>
          {collapsed ? 'IB' : 'IBCON Бюджет'}
        </div>
        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={[selectedKey]}
          style={{ background: IBCON_COLOR, borderRight: 0 }}
          items={menuItems}
          onClick={({ key }) => navigate(key)}
        />
      </Sider>

      <Layout>
        <Header style={{
          background: '#fff',
          padding: '0 24px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          borderBottom: '1px solid #f0f0f0',
        }}>
          <span
            style={{ cursor: 'pointer', fontSize: 18 }}
            onClick={() => setCollapsed(!collapsed)}
          >
            {collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
          </span>
          <Dropdown
            menu={{
              items: [
                {
                  key: 'logout',
                  icon: <LogoutOutlined />,
                  label: 'Выйти',
                  onClick: logout,
                },
              ],
            }}
          >
            <div style={{ cursor: 'pointer', display: 'flex', alignItems: 'center', gap: 8 }}>
              <Avatar style={{ background: IBCON_COLOR }} size="small">
                {user?.full_name?.[0] ?? 'U'}
              </Avatar>
              <Text style={{ fontSize: 13 }}>{user?.full_name}</Text>
            </div>
          </Dropdown>
        </Header>

        <Content style={{ margin: '24px 24px', minHeight: 280 }}>
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  );
}
