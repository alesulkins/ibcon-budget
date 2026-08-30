import React, { useState } from 'react';
import { Layout, Menu, Avatar, Dropdown, Typography, Breadcrumb, Drawer, Grid } from 'antd';
import {
  UserOutlined, LogoutOutlined, MenuFoldOutlined, MenuUnfoldOutlined,
  RightOutlined,
} from '@ant-design/icons';
import {
  IconProjects, IconReferences, IconUsers, IconHistory, IconSettings,
} from './SidebarIcons';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { profileApi, projectsApi, budgetsApi } from '../api';
import type { Profile, ProjectListItem, PaginatedResponse } from '../types';
import { ROLE_LABELS } from '../types';
import { shortName, initials } from '../utils/names';
import { useNavigate, useLocation, useMatch, Outlet } from 'react-router-dom';
import { clearAuth, currentUser } from '../store/auth';
import { PERM, usePermissions } from '../store/permissions';
import { useScrollRestore, SCROLL_ROOT_ID } from '../hooks/useScrollRestore';
import { SIDER_FOOTER_ID } from '../hooks/useFillHeight';
import ReminderPopups from './ReminderPopups';

const { Header, Sider, Content } = Layout;
const { Text } = Typography;

/**
 * Оформление панели — общее для обычного сайдбара и выдвижной панели на
 * телефоне: панель одна и та же, меняется только способ её показать.
 */
const SIDER_BACKGROUND = {
  background: 'var(--ibcon-sider-gradient)',
  backdropFilter: 'blur(14px)',
} as const;


/** Заголовок раздела в шапке — для экранов без своей цепочки крошек. */
const SECTION_TITLES: Record<string, string> = {
  '/projects': 'Проекты',
  '/references': 'Справочники',
  '/users': 'Пользователи',
  '/audit': 'История изменений',
  '/profile': 'Личный кабинет',
  '/settings': 'Настройки',
};

/**
 * Аватар пользователя: картинка, эмодзи-стикер или инициалы —
 * в таком порядке приоритета.
 */
function ProfileAvatar({ profile, fullName, size, on }: {
  profile?: Profile;
  fullName?: string;
  size: 'small' | 'default';
  /** Где стоит аватар: на панели или в светлой шапке. */
  on: 'sider' | 'header';
}) {
  const avatar = profile?.avatar ?? '';
  const isImage = avatar.startsWith('data:');
  return (
    <Avatar
      size={size}
      src={isImage ? avatar : undefined}
      style={{
        // На панели кружок должен быть заодно с блоком ЛК, а не синим
        // пятном: подсветка тем же полупрозрачным белым. В шапке фон
        // светлый, там нужна фирменная заливка.
        background: isImage
          ? undefined
          : on === 'sider' ? 'rgba(253, 249, 248, 0.16)' : 'var(--ibcon-brand)',
        flexShrink: 0,
      }}
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

  /**
   * Вид экрана. Телефон — уже 768 точек (antd md): на такой ширине
   * сайдбар отнимал бы половину экрана, поэтому там он выезжает поверх
   * страницы. Планшет (768–992) обходится свёрнутым сайдбаром — это
   * делает сам Sider по breakpoint="lg".
   */
  const screens = Grid.useBreakpoint();
  const mobile = !screens.md;
  const [drawerOpen, setDrawerOpen] = useState(false);

  // Смена страницы закрывает выдвижную панель: на телефоне она
  // перекрывает содержимое, и оставлять её открытой поверх нового
  // раздела незачем.
  const { pathname } = location;
  React.useEffect(() => { setDrawerOpen(false); }, [pathname]);

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

  // Разделы меню — по правам, а не по ролям: главный экономист может
  // выдать историю изменений или справочники индивидуально, и пункт
  // должен появиться сам.
  const { can } = usePermissions();

  const menuItems = [
    {
      key: '/projects',
      icon: <IconProjects />,
      label: 'Проекты',
    },
    {
      key: '/references',
      icon: <IconReferences />,
      label: 'Справочники',
    },
    ...(can(PERM.usersManage) ? [{
      key: '/users',
      icon: <IconUsers />,
      label: 'Пользователи',
    }] : []),
    ...(can(PERM.auditView) ? [{
      key: '/audit',
      icon: <IconHistory />,
      label: 'История изменений',
    }] : []),
    // Настройки интерфейса — свой раздел, а не подвал личного кабинета:
    // в них заходят отдельно от работы с профилем.
    {
      key: '/settings',
      icon: <IconSettings />,
      label: 'Настройки',
    },
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
    // Выбрали раздел — панель уходит, даже если это текущий раздел и
    // адрес не сменится.
    setDrawerOpen(false);
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

  // Содержимое панели одно и то же во всех трёх видах экрана —
  // отличается только способ её показать (см. ниже).
  // narrow — панель свёрнута в иконки. На телефоне такого вида нет:
  // выдвижная панель всегда развёрнута, иначе в ней остались бы одни
  // значки без подписей.
  const narrow = !mobile && collapsed;

  const panelBody = (
    <>
        {/* Логотип: развёрнутый знак, в свёрнутом виде — последняя буква.
            Оба файла залиты фирменным белым, поэтому читаются на панели
            без дополнительной обработки. */}
        <div style={{
          height: 64,
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          padding: narrow ? '0 12px' : '0 20px',
          // Разделитель в две грани: тёмная линия и светлый блик под ней —
          // так край читается как рельеф, а не как нарисованная черта.
          borderBottom: '1px solid rgba(0,0,0,0.18)',
          boxShadow: '0 1px 0 rgba(255,255,255,0.06)',
        }}>
          <img
            src={narrow ? '/logo-last-letter.svg' : '/logo.svg'}
            alt="IBCON"
            // Высота фиксирована, ширина считается по пропорции: знак
            // широкий (270×60), обрезанная буква почти квадратная.
            style={{
              height: narrow ? 28 : 24,
              width: 'auto',
              maxWidth: '100%',
              display: 'block',
            }}
          />
          {/* Название продукта под знаком: сам знак — марка компании,
              а систем у неё несколько. В свёрнутой панели не помещается. */}
          {!narrow && (
            <span style={{
              marginTop: 4,
              fontSize: 8,
              letterSpacing: '0.3em',
              textTransform: 'uppercase',
              color: 'rgba(253, 249, 248, 0.62)',
            }}>
              Бюджет
            </span>
          )}
        </div>
        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={[selectedKey]}
          // Прозрачное меню поверх растяжки сайдбара: со своей заливкой
          // оно ложилось ровным прямоугольником и гасило градиент.
          // inlineIndent по умолчанию 24 — при нём «История изменений»
          // не помещалась в ширину панели и обрезалась многоточием.
          inlineIndent={12}
          style={{ background: 'transparent', borderRight: 0, marginTop: 8 }}
          items={menuItems}
          onClick={({ key }) => onMenuClick(key)}
        />

        {/* Блок пользователя внизу сайдбара — вход в личный кабинет.
            id читает useFillHeight: верхняя грань этого блока — линия,
            ниже которой большим таблицам (реестр, пользователи, история)
            расти нельзя, дальше у них своя прокрутка. */}
        <div
          id={SIDER_FOOTER_ID}
          onClick={() => navigate('/profile')}
          title="Личный кабинет"
          style={{
            position: 'absolute',
            bottom: 0,
            left: 0,
            right: 0,
            padding: narrow ? '12px 8px' : '12px 16px',
            borderTop: '1px solid rgba(255,255,255,0.10)',
            // Матовая полка: подсветка сверху вниз плюс размытие того,
            // что за ней, — блок отделяется от панели, не разрезая её.
            backdropFilter: 'blur(10px)',
            display: 'flex',
            alignItems: 'center',
            // В свёрнутом виде остаётся один аватар — ставим его по центру
            // колонки, иначе он прижимается к левому краю.
            justifyContent: narrow ? 'center' : 'flex-start',
            gap: narrow ? 0 : 10,
            cursor: 'pointer',
            transition: 'background 0.2s ease',
            background: location.pathname === '/profile'
              ? 'rgba(255,255,255,0.14)'
              : 'transparent',
          }}
        >
          <ProfileAvatar
            profile={profile}
            fullName={user?.full_name}
            size={narrow ? 'small' : 'default'}
            on="sider"
          />
          {!narrow && (
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
    </>
  );

  return (
    <Layout style={{ height: '100vh', overflow: 'hidden' }}>
      {/* Панель навигации.

          На телефоне сайдбар не помещается: он съедал бы половину узкого
          экрана, а свёрнутый в иконки — оставлял бы человека без подписей.
          Поэтому там та же панель выезжает поверх страницы по кнопке в
          шапке и закрывается сразу после выбора раздела. На планшете
          сайдбар остаётся, но свёрнутым (breakpoint="lg"). */}
      {mobile ? (
        <Drawer
          placement="left"
          open={drawerOpen}
          onClose={() => setDrawerOpen(false)}
          width={240}
          closable={false}
          styles={{
            body: { padding: 0, position: 'relative', ...SIDER_BACKGROUND },
            header: { display: 'none' },
          }}
        >
          {panelBody}
        </Drawer>
      ) : (
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
        // Сайдбар стоит во всю высоту окна. Липким он был, пока
        // прокручивался документ; теперь прокручивается только рабочая
        // область, и двигать сайдбар нечему.
        //
        // Матовое стекло (выбор владельца 2026-08-27): полупрозрачная
        // растяжка фирменного цвета с размытием, светлая грань справа и
        // внутренний блик. Панель читается как стеклянная пластина над
        // страницей, а не как вырезанный из бумаги прямоугольник.
        style={{
          // Растяжку собирает ThemedApp из выбранного фирменного цвета:
          // литералами она не реагировала бы на смену цвета в настройках.
          background: 'var(--ibcon-sider-gradient)',
          backdropFilter: 'blur(14px)',
          height: '100%',
          borderRight: '1px solid rgba(253, 249, 248, 0.14)',
          boxShadow: 'inset -1px 0 0 rgba(253, 249, 248, 0.06),'
            + ' 4px 0 24px rgba(12, 26, 51, 0.07)',
          zIndex: 30,
        }}
        trigger={null}
      >
        {panelBody}
      </Sider>
      )}

      {/* minWidth: 0 — иначе широкая таблица растягивает колонку целиком
          и «выталкивает» сайдбар вместо того, чтобы прокручиваться. */}
      <Layout style={{
        minWidth: 0,
        height: '100%',
        display: 'flex',
        flexDirection: 'column',
        overflow: 'hidden',
      }}>
        {/* Шапка стоит вне прокручиваемой панели, поэтому не «липкая»:
            уезжать ей не от чего. */}
        {/* Фон и линия шапки — переменными, а не литералами: в тёмной
            теме белая полупрозрачная подложка оставляла тёмный текст на
            почти белом стекле. Переменные подменяет ThemedApp. */}
        <Header style={{
          background: 'var(--ibcon-header-bg)',
          backdropFilter: 'blur(12px)',
          padding: mobile ? '0 12px' : '0 24px',
          flexShrink: 0,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: 16,
          borderBottom: '1px solid var(--ibcon-line)',
          boxShadow: 'var(--ibcon-header-shadow)',
        }}>
          <div style={{
            display: 'flex',
            alignItems: 'center',
            gap: 16,
            minWidth: 0,
          }}>
            <span
              style={{ cursor: 'pointer', fontSize: 18, flexShrink: 0 }}
              // На телефоне та же кнопка выдвигает панель, а не
              // сворачивает сайдбар — сворачивать там нечего.
              onClick={() => (mobile ? setDrawerOpen(o => !o) : setCollapsed(!collapsed))}
            >
              {(mobile ? drawerOpen : !collapsed)
                ? <MenuFoldOutlined />
                : <MenuUnfoldOutlined />}
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
                      // Цвет переменной, а не литералом: в тёмной теме
                      // тёмно-серый заголовок пропадал на тёмной шапке.
                      color: c.title ? 'var(--ibcon-text)' : 'transparent',
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
              <ProfileAvatar
                profile={profile}
                fullName={user?.full_name}
                size="small"
                on="header"
              />
              {/* В шапке — сокращённое ФИО, полное живёт в ЛК. На
                  телефоне остаётся только кружок: имя вытесняло бы
                  цепочку крошек, а она нужнее — по ней возвращаются. */}
              {!mobile && (
                <Text style={{ fontSize: 13 }}>{shortName(user?.full_name)}</Text>
              )}
            </div>
          </Dropdown>
        </Header>

        {/* Напоминания догоняют человека на любой странице, поэтому
            живут в каркасе, а не в личном кабинете. */}
        <ReminderPopups />

        <Content
          id={SCROLL_ROOT_ID}
          className="ibcon-scroll-root ibcon-scroll"
          // На узком экране отступ рабочей области меньше: 24 точки с
          // каждой стороны съедали бы восьмую часть ширины телефона.
          style={{ padding: mobile ? 12 : 24, minWidth: 0 }}
        >
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  );
}
