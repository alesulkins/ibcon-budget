import React, { useState } from 'react';
import { Layout, Menu, Avatar, Dropdown, Typography, Breadcrumb } from 'antd';
import {
  ProjectOutlined, BookOutlined, UserOutlined,
  HistoryOutlined, LogoutOutlined, MenuFoldOutlined, MenuUnfoldOutlined,
  RightOutlined,
} from '@ant-design/icons';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { profileApi, projectsApi, budgetsApi } from '../api';
import type { Profile, ProjectListItem, PaginatedResponse } from '../types';
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

/**
 * Вспомогательные разделы: в них заходят посмотреть значение и
 * возвращаются к работе. Клик по «Проекты» из любого из них должен
 * вернуть туда, откуда ушли.
 */
const SERVICE_SECTIONS = ['/references', '/users', '/audit'];
const RETURN_TO_KEY = 'projects:returnTo';

/**
 * Экран конкретного проекта или версии бюджета — только с такого экрана
 * есть смысл возвращаться. Уход в справочники из самого реестра ничего
 * не запоминает: реестр и так открывается по «Проектам».
 */
function isWorkScreen(path: string): boolean {
  return /^\/projects\/\d+/.test(path) || /^\/budget-versions\/\d+/.test(path);
}

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

  /**
   * Название проекта для крошки.
   *
   * Пока запрос карточки в полёте, берём имя из уже загруженного реестра:
   * при переходе «реестр → проект» оно известно сразу, и подпись не
   * успевает мигнуть заглушкой. Если проекта в кэше нет (зашли по прямой
   * ссылке), показываем пустое место, а не другое слово — мигание чем-то
   * посторонним и было жалобой.
   */
  const qc = useQueryClient();
  const projectName = crumbProject?.name ?? (() => {
    if (!projectId) return '';
    const cached = qc.getQueriesData<PaginatedResponse<ProjectListItem>>({
      queryKey: ['projects'],
    });
    for (const [, data] of cached) {
      const hit = data?.items?.find(p => p.id === projectId);
      if (hit) return hit.name;
    }
    return '';
  })();

  // Последний элемент цепочки — текущий экран, он не ссылка.
  const crumbs: { title: string; to?: string }[] = (() => {
    if (budgetMatch) {
      return [
        { title: 'Проекты', to: '/projects' },
        {
          title: projectName,
          to: projectId ? `/projects/${projectId}` : undefined,
        },
        // ТЗ, таблица 4: бюджеты нумеруются «ID проекта.номер версии».
        {
          title: crumbVersion
            ? `Бюджет ${crumbVersion.project_id}.${crumbVersion.version_no}`
            : '',
        },
      ];
    }
    if (projectMatch) {
      return [
        { title: 'Проекты', to: '/projects' },
        { title: projectName },
      ];
    }
    return [{ title: SECTION_TITLES[selectedKey] ?? 'Проекты' }];
  })();

  /**
   * Возврат из вспомогательного раздела туда, откуда пользователь ушёл.
   *
   * Правило узкое и срабатывает только по цепочке «экран проекта или
   * версии бюджета → справочники / пользователи / история → Проекты».
   * Обычное хождение по пунктам меню (реестр → справочники → Проекты)
   * возврата не включает: там возвращаться не к чему, и открывать вместо
   * реестра давнюю карточку было бы неожиданно. Адрес запоминаем в
   * момент ухода — при возврате он уже потерян.
   */
  function onMenuClick(key: string) {
    const from = location.pathname + location.search;
    const inService = SERVICE_SECTIONS.includes(selectedKey);

    if (SERVICE_SECTIONS.includes(key)) {
      try {
        if (isWorkScreen(from)) {
          sessionStorage.setItem(RETURN_TO_KEY, from);
        } else if (!inService) {
          // Пришли не с рабочего экрана — прошлую метку гасим, иначе она
          // сработала бы много позже и не к месту. Переход между самими
          // вспомогательными разделами метку сохраняет.
          sessionStorage.removeItem(RETURN_TO_KEY);
        }
      } catch { /* приватный режим — просто не запомним */ }
      navigate(key);
      return;
    }

    if (key === '/projects') {
      let back: string | null = null;
      try {
        back = sessionStorage.getItem(RETURN_TO_KEY);
        sessionStorage.removeItem(RETURN_TO_KEY);
      } catch { /* нет доступа к хранилищу — уйдём в реестр */ }

      if (inService && back && isWorkScreen(back)) {
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
        //
        // Вместо плоской заливки — вертикальная растяжка от светлого верха
        // к тёмному низу, тонкая светлая грань справа и мягкая тень на
        // контент. Панель перестаёт выглядеть наклейкой и получает объём.
        style={{
          background: `linear-gradient(170deg, #24487f 0%, ${IBCON_COLOR} 42%, #14294c 100%)`,
          position: 'sticky',
          top: 0,
          height: '100vh',
          borderRight: '1px solid rgba(255,255,255,0.08)',
          boxShadow: '3px 0 18px rgba(12, 26, 51, 0.16)',
          zIndex: 30,
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
          // Разделитель в две грани: тёмная линия и светлый блик под ней —
          // так край читается как рельеф, а не как нарисованная черта.
          borderBottom: '1px solid rgba(0,0,0,0.18)',
          boxShadow: '0 1px 0 rgba(255,255,255,0.06)',
        }}>
          {collapsed ? 'IB' : 'IBCON Бюджет'}
        </div>
        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={[selectedKey]}
          // Прозрачное меню поверх растяжки сайдбара: со своей заливкой
          // оно ложилось ровным прямоугольником и гасило градиент.
          style={{ background: 'transparent', borderRight: 0, marginTop: 8 }}
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
            borderTop: '1px solid rgba(255,255,255,0.10)',
            // Матовая полка: подсветка сверху вниз плюс размытие того,
            // что за ней, — блок отделяется от панели, не разрезая её.
            backdropFilter: 'blur(10px)',
            display: 'flex',
            alignItems: 'center',
            // В свёрнутом виде остаётся один аватар — ставим его по центру
            // колонки, иначе он прижимается к левому краю.
            justifyContent: collapsed ? 'center' : 'flex-start',
            gap: collapsed ? 0 : 10,
            cursor: 'pointer',
            transition: 'background 0.2s ease',
            background: location.pathname === '/profile'
              ? 'rgba(255,255,255,0.14)'
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
        {/* Шапка липкая: раньше при прокрутке она уезжала и на её месте
            обнажался край рабочей области. */}
        <Header style={{
          background: 'rgba(255,255,255,0.86)',
          backdropFilter: 'blur(12px)',
          padding: '0 24px',
          position: 'sticky',
          top: 0,
          zIndex: 20,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: 16,
          borderBottom: '1px solid rgba(0,0,0,0.06)',
          boxShadow: '0 1px 2px rgba(16, 30, 54, 0.04)',
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
                // Пустая подпись — данные ещё грузятся. Держим место
                // прочерком: подставлять слово-заглушку нельзя, иначе оно
                // мигнёт вместо названия проекта.
                title: c.to
                  ? <a onClick={() => navigate(c.to!)}>{c.title || '…'}</a>
                  : (
                    <span style={{
                      color: c.title ? '#262626' : 'transparent',
                      fontWeight: 500,
                    }}>
                      {c.title || '…'}
                    </span>
                  ),
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
