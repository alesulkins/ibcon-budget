import React, { useState } from 'react';
import { Layout, Menu, Avatar, Dropdown, Typography } from 'antd';
import {
  ProjectOutlined, BookOutlined, UserOutlined,
  HistoryOutlined, LogoutOutlined, MenuFoldOutlined, MenuUnfoldOutlined,
  RightOutlined,
} from '@ant-design/icons';
import { useQuery } from '@tanstack/react-query';
import { profileApi } from '../api';
import type { Profile } from '../types';
import { ROLE_LABELS } from '../types';
import { shortName, initials } from '../utils/names';
import { useNavigate, useLocation, Outlet } from 'react-router-dom';
import { clearAuth, currentUser, hasRole } from '../store/auth';
import { useScrollRestore } from '../hooks/useScrollRestore';

const { Header, Sider, Content } = Layout;
const { Text } = Typography;

const IBCON_COLOR = '#1a3a6b';

/**
 * Аватар пользователя: картинка, эмодзи-стикер или инициалы —
 * в таком порядке приоритета.
 */
function ProfileAvatar({ profile, fullName, size }: {
  profile?: Profile;
  fullName?: string;
  size: 'small' | 'default';
}) {
  const avatar = profile?.avatar ?? '';
  const isImage = avatar.startsWith('data:');
  return (
    <Avatar
      size={size}
      src={isImage ? avatar : undefined}
      style={{ background: isImage ? undefined : '#3a5f9e', flexShrink: 0 }}
    >
      {!isImage && (avatar || initials(fullName ?? profile?.full_name))}
    </Avatar>
  );
}

/** Раздел, из которого умеем возвращаться на прежний экран. */
const RETURNABLE_FROM = '/references';
const RETURN_TO_KEY = 'references:returnTo';

export default function AppLayout() {
  const navigate = useNavigate();
  const location = useLocation();
  const user = currentUser();
  const [collapsed, setCollapsed] = useState(false);

  // Возврат из «Справочников» не должен выбрасывать в начало страницы
  useScrollRestore();

  // Аватар из профиля — показываем в сайдбаре и шапке
  const { data: profile } = useQuery({
    queryKey: ['profile'],
    queryFn: profileApi.get,
  });

  function logout() {
    // Email последнего входа сохраняем — «Запомнить меня» подставит его
    // в форму при следующем входе.
    clearAuth({ keepSavedEmail: true });
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

  /**
   * Возврат из справочников туда, откуда пользователь в них ушёл.
   *
   * «Справочники» — вспомогательный раздел: в него заходят посмотреть
   * значение и возвращаются к работе. Клик по «Проекты» должен вернуть
   * на конкретный экран (карточку, шаг мастера), а не на реестр.
   * Адрес запоминаем в момент ухода, а не при возврате — иначе он уже
   * потерян.
   */
  function onMenuClick(key: string) {
    if (key === RETURNABLE_FROM && selectedKey !== RETURNABLE_FROM) {
      // Уходим в справочники — запомним, откуда.
      try {
        sessionStorage.setItem(RETURN_TO_KEY, location.pathname + location.search);
      } catch { /* приватный режим — просто не запомним */ }
      navigate(key);
      return;
    }

    if (key === '/projects' && selectedKey === RETURNABLE_FROM) {
      // Возвращаемся из справочников — если есть куда, идём туда.
      let back: string | null = null;
      try {
        back = sessionStorage.getItem(RETURN_TO_KEY);
        sessionStorage.removeItem(RETURN_TO_KEY);
      } catch { /* нет доступа к хранилищу — уйдём в реестр */ }

      if (back && back.startsWith('/') && back !== RETURNABLE_FROM) {
        navigate(back);
        return;
      }
    }

    navigate(key);
  }

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
          onClick={({ key }) => onMenuClick(key)}
        />

        {/* Блок пользователя внизу сайдбара — вход в личный кабинет */}
        <div
          onClick={() => navigate('/profile')}
          title="Личный кабинет"
          style={{
            position: 'absolute',
            bottom: 0,
            left: 0,
            right: 0,
            padding: collapsed ? '12px 8px' : '12px 16px',
            borderTop: '1px solid rgba(255,255,255,0.15)',
            display: 'flex',
            alignItems: 'center',
            gap: 10,
            cursor: 'pointer',
            background: location.pathname === '/profile'
              ? 'rgba(255,255,255,0.12)'
              : 'transparent',
          }}
        >
          <ProfileAvatar profile={profile} fullName={user?.full_name} size={collapsed ? 'small' : 'default'} />
          {!collapsed && (
            <>
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{
                  color: '#fff',
                  fontSize: 13,
                  whiteSpace: 'nowrap',
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                }}>
                  {shortName(user?.full_name)}
                </div>
                <div style={{ color: 'rgba(255,255,255,0.6)', fontSize: 11 }}>
                  {ROLE_LABELS[user?.role ?? ''] ?? user?.role}
                </div>
              </div>
              <RightOutlined style={{ color: 'rgba(255,255,255,0.6)', fontSize: 11 }} />
            </>
          )}
        </div>
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
                  key: 'profile',
                  icon: <UserOutlined />,
                  label: 'Личный кабинет',
                  onClick: () => navigate('/profile'),
                },
                { type: 'divider' },
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
              <ProfileAvatar profile={profile} fullName={user?.full_name} size="small" />
              {/* В шапке — сокращённое ФИО, полное живёт в ЛК */}
              <Text style={{ fontSize: 13 }}>{shortName(user?.full_name)}</Text>
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
