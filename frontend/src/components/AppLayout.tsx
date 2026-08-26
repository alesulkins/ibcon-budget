import React, { useState } from 'react';
import { Layout, Menu, Avatar, Dropdown, Typography, Breadcrumb } from 'antd';
import {
  ProjectOutlined, BookOutlined, UserOutlined,
  HistoryOutlined, LogoutOutlined, MenuFoldOutlined, MenuUnfoldOutlined,
  RightOutlined,
} from '@ant-design/icons';
import { useQuery } from '@tanstack/react-query';
import { profileApi, projectsApi, budgetsApi } from '../api';
import type { Profile } from '../types';
import { ROLE_LABELS } from '../types';
import { shortName, initials } from '../utils/names';
import { useNavigate, useLocation, useMatch, Outlet } from 'react-router-dom';
import { clearAuth, currentUser, hasRole } from '../store/auth';
import { useScrollRestore } from '../hooks/useScrollRestore';

const { Header, Sider, Content } = Layout;
const { Text } = Typography;

const IBCON_COLOR = '#1a3a6b';

/** Заголовок раздела в шапке — для экранов без своей цепочки крошек. */
const SECTION_TITLES: Record<string, string> = {
  '/projects': 'Проекты',
  '/references': 'Справочники',
  '/users': 'Пользователи',
  '/audit': 'История изменений',
  '/profile': 'Личный кабинет',
};

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
   * Хлебные крошки в шапке.
   *
   * Название проекта и номер бюджета берём теми же ключами запросов, что
   * и сами страницы, — react-query отдаёт их из кэша, лишних запросов
   * шапка не делает.
   */
  const projectMatch = useMatch('/projects/:id');
  const budgetMatch = useMatch('/budget-versions/:vid');

  const versionId = budgetMatch ? Number(budgetMatch.params.vid) : undefined;
  const { data: crumbVersion } = useQuery({
    queryKey: ['budget-version', versionId],
    queryFn: () => budgetsApi.getVersion(versionId!),
    enabled: !!versionId,
  });

  const projectId = projectMatch ? Number(projectMatch.params.id) : crumbVersion?.project_id;
  const { data: crumbProject } = useQuery({
    queryKey: ['project', projectId],
    queryFn: () => projectsApi.get(projectId!),
    enabled: !!projectId,
  });

  // Последний элемент цепочки — текущий экран, он не ссылка.
  const crumbs: { title: string; to?: string }[] = (() => {
    if (budgetMatch) {
      return [
        { title: 'Проекты', to: '/projects' },
        {
          title: crumbProject?.name ?? 'Проект',
          to: projectId ? `/projects/${projectId}` : undefined,
        },
        // ТЗ, таблица 4: бюджеты нумеруются «ID проекта.номер версии».
        {
          title: crumbVersion
            ? `Бюджет ${crumbVersion.project_id}.${crumbVersion.version_no}`
            : 'Бюджет',
        },
      ];
    }
    if (projectMatch) {
      return [
        { title: 'Проекты', to: '/projects' },
        { title: crumbProject?.name ?? 'Проект' },
      ];
    }
    return [{ title: SECTION_TITLES[selectedKey] ?? 'Проекты' }];
  })();

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
        // На узком экране сайдбар сворачивается сам — ровно так же, как
        // от кнопки в шапке: содержимое страницы получает всю ширину и
        // таблицы не выдавливают вёрстку.
        breakpoint="lg"
        onBreakpoint={setCollapsed}
        theme="dark"
        // Липкий сайдбар на всю высоту экрана: длинная страница мастера
        // раньше уводила блок ЛК (он прижат к низу) в самый низ документа.
        style={{
          background: IBCON_COLOR,
          position: 'sticky',
          top: 0,
          height: '100vh',
        }}
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
            // В свёрнутом виде остаётся один аватар — ставим его по центру
            // колонки, иначе он прижимается к левому краю.
            justifyContent: collapsed ? 'center' : 'flex-start',
            gap: collapsed ? 0 : 10,
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

      {/* minWidth: 0 — иначе широкая таблица растягивает колонку целиком
          и «выталкивает» сайдбар вместо того, чтобы прокручиваться. */}
      <Layout style={{ minWidth: 0 }}>
        <Header style={{
          background: '#fff',
          padding: '0 24px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: 16,
          borderBottom: '1px solid #f0f0f0',
        }}>
          <div style={{
            display: 'flex',
            alignItems: 'center',
            gap: 16,
            minWidth: 0,
          }}>
            <span
              style={{ cursor: 'pointer', fontSize: 18, flexShrink: 0 }}
              onClick={() => setCollapsed(!collapsed)}
            >
              {collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
            </span>
            <Breadcrumb
              style={{ fontSize: 14, minWidth: 0 }}
              items={crumbs.map((c, i) => ({
                title: c.to
                  ? <a onClick={() => navigate(c.to!)}>{c.title}</a>
                  : <span style={{ color: '#262626', fontWeight: 500 }}>{c.title}</span>,
                key: i,
              }))}
            />
          </div>
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
            <div style={{
              cursor: 'pointer', display: 'flex', alignItems: 'center',
              gap: 8, flexShrink: 0,
            }}>
              <ProfileAvatar profile={profile} fullName={user?.full_name} size="small" />
              {/* В шапке — сокращённое ФИО, полное живёт в ЛК */}
              <Text style={{ fontSize: 13 }}>{shortName(user?.full_name)}</Text>
            </div>
          </Dropdown>
        </Header>

        <Content style={{ margin: '24px 24px', minHeight: 280, minWidth: 0 }}>
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  );
}
