import { useMemo, useState } from 'react';
import { Table, Button, Space, Modal, Form, Input, Select, message } from 'antd';
import { PlusOutlined, UserOutlined } from '@ant-design/icons';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import type { ColumnsType } from 'antd/es/table';
import { usersApi, projectsApi } from '../../api';
import type { User } from '../../types';
import { ROLES, ROLE_LABELS } from '../../types';
import { extractError } from '../../api/client';
import { PERMISSION_LABELS, PERM_ALL, hasFullAccessByRole } from '../../store/permissions';
import { shortName } from '../../utils/names';
import StatusTag from '../../components/StatusTag';
import UserPanel from './UserPanel';
import { useFillToSiderFooter } from '../../hooks/useFillHeight';

/** Высота подвала пагинации antd (small) — в высоту таблицы не входит:
 *  `scroll.y` ограничивает только тело таблицы, подвал рисуется под ним. */
const PAGINATION_RESERVE = 64;

// Порядок ролей в списке — от самой полной к самой узкой, не по алфавиту:
// так сортировка по роли читается как «сверху те, кто может больше».
const ROLE_ORDER = ['GE', 'EP', 'RP', 'AP', 'IP', 'MANAGEMENT', ''];

function roleRank(role: string): number {
  const i = ROLE_ORDER.indexOf(role);
  return i === -1 ? ROLE_ORDER.length : i;
}

export default function UsersPage() {
  const qc = useQueryClient();
  const [search, setSearch] = useState('');
  const [roleFilter, setRoleFilter] = useState<string | undefined>();
  const [showCreate, setShowCreate] = useState(false);
  const [panelUser, setPanelUser] = useState<User | null>(null);
  const [createForm] = Form.useForm();
  const [fillRef, fillHeight] = useFillToSiderFooter<HTMLDivElement>(PAGINATION_RESERVE);

  const { data: users, isLoading } = useQuery({
    queryKey: ['users'],
    queryFn: usersApi.list,
  });

  const { data: projectsData } = useQuery({
    queryKey: ['projects-all'],
    queryFn: () => projectsApi.list({ limit: 200 }),
  });

  // Поиск идёт и по ФИО, и по названию роли: «экономист» в строке поиска
  // должен находить и Ярулину, и всех главных экономистов сразу.
  const rows = useMemo(() => {
    const q = search.trim().toLowerCase();
    return (users ?? [])
      .filter(u => {
        if (roleFilter !== undefined && u.role !== roleFilter) return false;
        if (!q) return true;
        const role = ROLE_LABELS[u.role] ?? u.role;
        return `${u.full_name} ${role} ${u.email}`.toLowerCase().includes(q);
      })
      .sort((a, b) => {
        if (a.active !== b.active) return a.active ? -1 : 1;
        return a.full_name.localeCompare(b.full_name, 'ru');
      });
  }, [users, search, roleFilter]);

  // Панель держит пользователя по id, а не копию объекта: после
  // сохранения список перезапрашивается, и копия устарела бы —
  // выданное право не появилось бы в панели до её переоткрытия.
  const openUser = panelUser
    ? (users ?? []).find(u => u.id === panelUser.id) ?? panelUser
    : null;

  const createMutation = useMutation({
    mutationFn: (vals: { email: string; full_name: string; role: string; password: string }) =>
      usersApi.create(vals),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['users'] });
      message.success('Пользователь создан');
      setShowCreate(false);
      createForm.resetFields();
    },
    onError: (e) => message.error(extractError(e)),
  });

  const roleOptions = [
    { value: '', label: ROLE_LABELS[''] },
    ...Object.values(ROLES).map((r) => ({ value: r, label: ROLE_LABELS[r] })),
  ];

  /**
   * Любая сортировка сначала разводит активных и отключённых, и только
   * потом сравнивает по самой колонке: отключённая учётка не должна
   * всплывать наверх из-за фамилии или роли.
   */
  const byActiveThen = (cmp: (a: User, b: User) => number) =>
    (a: User, b: User) => (a.active !== b.active ? (a.active ? -1 : 1) : cmp(a, b));

  const columns: ColumnsType<User> = [
    {
      title: 'ФИО',
      dataIndex: 'full_name',
      // В таблице — сокращённо «Фамилия И.О.»; полное ФИО живёт в панели.
      render: (v: string) => shortName(v),
      sorter: byActiveThen((a, b) => a.full_name.localeCompare(b.full_name, 'ru')),
    },
    { title: 'Email', dataIndex: 'email' },
    {
      title: 'Роль',
      dataIndex: 'role',
      // Сортировка по «весу» роли, а не по алфавиту, — см. ROLE_ORDER.
      sorter: byActiveThen((a, b) => roleRank(a.role) - roleRank(b.role)),
      render: (r: string) => (
        <StatusTag color={r ? undefined : 'grey'}>
          {ROLE_LABELS[r] ?? r}
        </StatusTag>
      ),
    },
    {
      title: 'Статус',
      dataIndex: 'active',
      sorter: byActiveThen((a, b) => b.failed_attempts - a.failed_attempts),
      render: (active: boolean, row: User) => (
        <Space>
          <StatusTag color={active ? 'green' : 'grey'}>
            {active ? 'Активен' : 'Неактивен'}
          </StatusTag>
          {row.failed_attempts >= 5 && (
            <StatusTag color="amber">Много попыток входа</StatusTag>
          )}
        </Space>
      ),
    },
    {
      title: 'Доступ к проектам',
      key: 'projects',
      render: (_, row) => {
        // У главного экономиста доступ ко всем проектам идёт от роли, а
        // не от списка назначений: показываем факт, а не пустой прочерк.
        if (hasFullAccessByRole(row.role)) {
          return <StatusTag color="teal">Все проекты</StatusTag>;
        }
        const list = row.projects ?? [];
        if (list.length === 0) {
          return <span style={{ opacity: 0.45 }}>—</span>;
        }
        // Каждый проект с новой строки и ссылкой на его карточку.
        return (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
            {list.map(p => (
              <Link
                key={p.project_id}
                to={`/projects/${p.project_id}`}
                className="ibcon-link-plain"
                title={p.can_edit ? 'С правом редактирования' : 'Только просмотр'}
              >
                {p.name}
                {!p.can_edit && <span style={{ opacity: 0.5 }}> · просмотр</span>}
              </Link>
            ))}
          </div>
        );
      },
    },
    {
      title: 'Доп. права',
      key: 'grants',
      render: (_, row) => {
        // Все 14 прав у главного экономиста есть от роли — выдавать
        // сверх неё нечего.
        if (hasFullAccessByRole(row.role)) {
          return <StatusTag color="teal">Все права</StatusTag>;
        }
        const list = row.grants ?? [];
        if (list.length === 0) {
          return <span style={{ opacity: 0.45 }}>—</span>;
        }
        return (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
            {list.map(g => (
              <span key={`${g.permission}:${g.project_id ?? 'all'}`} style={{ fontSize: 12 }}>
                {g.permission === PERM_ALL
                  ? 'Все права'
                  : PERMISSION_LABELS[g.permission] ?? g.permission}
                <span style={{ opacity: 0.55 }}>
                  {' · '}
                  {g.project_id === null
                    ? 'все проекты'
                    : (g.project_name ?? `проект №${g.project_id}`)}
                </span>
              </span>
            ))}
          </div>
        );
      },
    },
    {
      title: '',
      key: 'actions',
      width: 56,
      align: 'right',
      // Колонки «доступ к проектам» и «доп. права» разгоняют таблицу
      // шире экрана, поэтому кнопку закрепляем справа — иначе до неё
      // пришлось бы доскроллить.
      fixed: 'right',
      render: (_, row) => (
        <Button
          size="small"
          icon={<UserOutlined />}
          title="Действия с пользователем"
          onClick={() => setPanelUser(row)}
        />
      ),
    },
  ];

  return (
    <div>
      {/* Название раздела живёт в шапке (AppLayout).
          Поиск и фильтр слева, кнопка создания — справа, как в реестре
          проектов: одинаковая раскладка у всех списков. */}
      {/* Поля сжимаются, кнопка — нет: с жёсткими ширинами на крупном
          шрифте строка переставала помещаться и кнопка съезжала вниз. */}
      <div className="ibcon-filters" style={{
        display: 'flex', gap: 12, alignItems: 'center',
        marginBottom: 12,
      }}>
        <Input.Search
          allowClear
          placeholder="Поиск по ФИО или роли"
          style={{ flex: '2 1 200px', minWidth: 150 }}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <Select
          allowClear
          placeholder="Все роли"
          style={{ flex: '1 1 160px', minWidth: 130 }}
          value={roleFilter}
          onChange={(v) => setRoleFilter(v)}
          options={roleOptions}
        />
        <Button
          type="primary" icon={<PlusOutlined />}
          style={{ marginLeft: 'auto', flexShrink: 0 }}
          onClick={() => { setShowCreate(true); createForm.resetFields(); }}
        >
          Создать пользователя
        </Button>
      </div>

      {/* scroll.y ограничивает тело таблицы высотой до линии ЛК в
          сайдбаре (useFillToSiderFooter) — страница не растёт вниз,
          прокручиваются только строки. */}
      <div ref={fillRef}>
        <Table
          rowKey="id"
          columns={columns}
          dataSource={rows}
          loading={isLoading}
          size="small"
          pagination={{ pageSize: 20 }}
          scroll={{ x: 'max-content', y: fillHeight }}
        />
      </div>

      <UserPanel
        user={openUser}
        projects={projectsData?.items ?? []}
        onClose={() => setPanelUser(null)}
      />

      <Modal
        title="Новый пользователь" open={showCreate}
        onCancel={() => { setShowCreate(false); createForm.resetFields(); }}
        onOk={() => createForm.submit()} okText="Создать" cancelText="Отмена"
        confirmLoading={createMutation.isPending}
      >
        <Form
          form={createForm}
          layout="vertical"
          onFinish={createMutation.mutate}
          initialValues={{ role: '' }}
        >
          <Form.Item
            name="full_name"
            label="ФИО полностью"
            rules={[{ required: true, message: 'Не заполнено обязательное поле: ФИО' }]}
            extra="Фамилия Имя Отчество. В таблицах показывается сокращённо: Фамилия И.О."
          >
            <Input placeholder="Иванов Иван Иванович" />
          </Form.Item>
          <Form.Item name="email" label="Email" rules={[{ required: true }, { type: 'email' }]}>
            <Input />
          </Form.Item>
          <Form.Item
            name="role"
            label="Роль"
            extra="Можно создать без роли — тогда пользователь войдёт, но разделов не увидит."
          >
            <Select options={roleOptions} />
          </Form.Item>
          <Form.Item
            name="password"
            label="Пароль"
            rules={[{ required: true, min: 10, message: 'Минимум 10 символов' }]}
          >
            <Input.Password />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
