import { useEffect, useMemo, useState } from 'react';
import {
  Drawer, Form, Input, Select, Switch, Button, Space,
  Typography, message,
} from 'antd';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { usersApi } from '../../api';
import { extractError } from '../../api/client';
import type { ProjectListItem, User } from '../../types';
import { ROLES, ROLE_LABELS } from '../../types';
import {
  PERM, PERM_ALL, PROJECT_PERMISSION_OPTIONS, hasFullAccessByRole,
} from '../../store/permissions';
import StatusTag from '../../components/StatusTag';
import { LINE, TEXT_SOFT } from '../../theme';

const { Text } = Typography;

interface Props {
  user: User | null;
  projects: ProjectListItem[];
  onClose: () => void;
}

/**
 * Панель действий над пользователем — всё, что главный экономист может
 * сделать с учёткой, в одном месте: ФИО, роль, статус, доступ к
 * проектам, пароль.
 *
 * Раньше на каждое действие была своя иконка в строке таблицы, и
 * половина возможностей (индивидуальные права, снятие роли, отзыв
 * доступа) не имела интерфейса вовсе. Индивидуальные права были и
 * отдельным блоком панели — теперь это одна колонка справа от проекта
 * в «Доступ к проектам»: править видно то же самое, что действует.
 */
export default function UserPanel({ user, projects, onClose }: Props) {
  const qc = useQueryClient();
  const [form] = Form.useForm();
  const [pwdForm] = Form.useForm();

  // Выбранные проекты и право на каждом. Локальное состояние:
  // пользователь отмечает несколько проектов и сохраняет одной кнопкой.
  const [selected, setSelected] = useState<number[]>([]);
  const [permByProject, setPermByProject] = useState<Record<number, string>>({});

  useEffect(() => {
    if (!user) return;
    form.setFieldsValue({
      full_name: user.full_name,
      email: user.email,
      role: user.role,
      active: user.active,
    });

    setSelected((user.projects ?? []).map(p => p.project_id));

    // Значение выпадающего списка берём из индивидуального гранта на
    // этот проект: «Все права» побеждает, если грантов несколько.
    // У строк без гранта (назначены до объединения панели в одну)
    // роли нет — грубая эвристика по старому флажку can_edit: включён
    // трактуем как «Изменение версии бюджета», выключен — как просмотр.
    const perms: Record<number, string> = {};
    for (const p of user.projects ?? []) {
      const grants = (user.grants ?? []).filter(g => g.project_id === p.project_id);
      if (grants.some(g => g.permission === PERM_ALL)) {
        perms[p.project_id] = PERM_ALL;
      } else if (grants.length > 0) {
        perms[p.project_id] = grants[0].permission;
      } else {
        perms[p.project_id] = p.can_edit ? PERM.budgetEdit : PERM.projectView;
      }
    }
    setPermByProject(perms);
  }, [user, form]);

  const invalidate = () => qc.invalidateQueries({ queryKey: ['users'] });

  const saveMutation = useMutation({
    mutationFn: (vals: { full_name: string; email: string; role: string; active: boolean }) =>
      usersApi.update(user!.id, vals),
    onSuccess: () => { invalidate(); message.success('Сохранено'); },
    onError: (e) => message.error(extractError(e)),
  });

  const saveAccessMutation = useMutation({
    mutationFn: async () => {
      // 1. Список проектов и грубый флаг can_edit — держит колонку
      //    «Доступ к проектам» в таблице пользователей и работу тех
      //    ролей, чьи базовые права зависят от самого назначения на
      //    проект (не от индивидуального гранта). Бэкенд сам отзывает
      //    индивидуальные права на проекты, пропавшие из списка.
      await usersApi.setProjects(
        user!.id,
        selected.map(id => ({
          project_id: id,
          can_edit: permByProject[id] !== PERM.projectView,
        })),
      );

      // 2. Точное право на каждый оставшийся проект — какое выбрано в
      //    выпадающем списке. Лишние гранты (после смены выбора)
      //    отзываем, нужный при отсутствии выдаём: индивидуальный
      //    грант — единственный способ выдать право, которого нет в
      //    базовой роли (например, согласование бюджета).
      for (const id of selected) {
        const want = permByProject[id] ?? PERM.projectView;
        const have = (user!.grants ?? []).filter(g => g.project_id === id);
        for (const g of have) {
          if (g.permission !== want) {
            await usersApi.revokePermission(user!.id, g.permission, id);
          }
        }
        if (!have.some(g => g.permission === want)) {
          await usersApi.grantPermission(user!.id, want, id);
        }
      }
    },
    onSuccess: () => { invalidate(); message.success('Доступ к проектам обновлён'); },
    onError: (e) => message.error(extractError(e)),
  });

  const pwdMutation = useMutation({
    mutationFn: (vals: { password: string }) => usersApi.setPassword(user!.id, vals.password),
    onSuccess: () => { message.success('Пароль изменён'); pwdForm.resetFields(); },
    onError: (e) => message.error(extractError(e)),
  });

  const unlockMutation = useMutation({
    mutationFn: () => usersApi.unlock(user!.id),
    onSuccess: () => { invalidate(); message.success('Блокировка снята'); },
    onError: (e) => message.error(extractError(e)),
  });

  const projectOptions = useMemo(
    () => projects.map(p => ({ value: p.id, label: `${p.id} · ${p.name}` })),
    [projects],
  );

  // Роль «не назначена» — обычный пункт списка, а не отсутствие выбора:
  // снять роль так же нужно, как назначить.
  const roleOptions = [
    { value: '', label: ROLE_LABELS[''] },
    ...Object.values(ROLES).map(r => ({ value: r, label: ROLE_LABELS[r] })),
  ];

  /**
   * Роль берём из формы, а не из сохранённой учётки: если в поле «Роль»
   * только что выбрали главного экономиста, блок доступа должен сразу
   * показать, что выбирать больше нечего, — ещё до сохранения.
   */
  const roleInForm = Form.useWatch('role', form) ?? user?.role ?? '';
  const fullAccess = hasFullAccessByRole(roleInForm);

  if (!user) return null;

  return (
    <Drawer
      open={!!user}
      onClose={onClose}
      width={520}
      title={user.full_name}
      styles={{ body: { paddingTop: 12 } }}
    >
      <Section title="Учётная запись">
        <Form
          form={form}
          layout="vertical"
          onFinish={saveMutation.mutate}
          initialValues={{
            full_name: user.full_name, email: user.email,
            role: user.role, active: user.active,
          }}
        >
          <Form.Item
            name="full_name"
            label="ФИО полностью"
            rules={[{ required: true, message: 'Не заполнено обязательное поле: ФИО' }]}
            extra="Фамилия Имя Отчество. В таблицах показывается сокращённо: Фамилия И.О."
          >
            <Input placeholder="Иванов Иван Иванович" />
          </Form.Item>
          <Form.Item
            name="email"
            label="Email"
            rules={[
              { required: true, message: 'Не заполнено обязательное поле: email' },
              { type: 'email', message: 'Похоже, это не адрес почты' },
            ]}
            extra="Это же логин для входа. Сам пользователь адрес не меняет: подмена почты была бы подменой входа."
          >
            <Input />
          </Form.Item>
          <Form.Item
            name="role"
            label="Роль"
            extra="Без роли пользователь входит в систему, но не видит разделов."
          >
            <Select options={roleOptions} />
          </Form.Item>
          <Form.Item
            name="active"
            label="Доступ"
            valuePropName="checked"
            extra="Отключённый пользователь не входит в систему; его действия остаются в истории изменений."
          >
            <Switch checkedChildren="Активен" unCheckedChildren="Отключён" />
          </Form.Item>
          <Space>
            <Button type="primary" htmlType="submit" loading={saveMutation.isPending}>
              Сохранить
            </Button>
            {user.failed_attempts >= 5 && (
              <Button onClick={() => unlockMutation.mutate()} loading={unlockMutation.isPending}>
                Снять блокировку входа
              </Button>
            )}
          </Space>
        </Form>
      </Section>

      <Section title="Доступ к проектам">
        {fullAccess ? (
          /* Главный экономист: все права на всех проектах идут от роли,
             выдавать сверх неё нечего. Показываем как факт — редактировать
             тут нечего, и бэкенд от роли зависит независимо от этой панели. */
          <>
            <Space size={8} style={{ marginBottom: 8 }}>
              <StatusTag color="teal">Все проекты</StatusTag>
              <StatusTag color="teal">Все права</StatusTag>
            </Space>
            <div>
              <Text type="secondary" style={{ fontSize: 12 }}>
                Главный экономист работает со всеми проектами и всеми
                действиями по роли. Отдельный доступ и права сверх роли ему
                не назначаются. Чтобы ограничить доступ — смените роль выше.
              </Text>
            </div>
          </>
        ) : (
        <>
        <Text type="secondary" style={{ fontSize: 12 }}>
          Справа от проекта — право на нём: от простого просмотра до всех
          действий. Что не отмечено выше — будет отозвано вместе с правом
          на этот проект.
        </Text>
        <Select
          mode="multiple"
          value={selected}
          onChange={(ids: number[]) => {
            setSelected(ids);
            // У новых проектов в списке права ещё нет — по умолчанию
            // просмотр, самый безопасный вариант.
            setPermByProject(prev => {
              const next = { ...prev };
              for (const id of ids) if (!(id in next)) next[id] = PERM.projectView;
              return next;
            });
          }}
          options={projectOptions}
          placeholder="Выберите проекты"
          style={{ width: '100%', marginTop: 8 }}
          showSearch
          filterOption={(inp, opt) => String(opt?.label).toLowerCase().includes(inp.toLowerCase())}
        />

        {/* Выбрать все разом: проектов в реестре десятки, и отмечать их
            по одному ради «доступ ко всему» — отдельная работа. Права на
            каждом проекте остаются прежними, новым ставится просмотр. */}
        <div style={{ marginTop: 8 }}>
          <Button
            size="small"
            type="link"
            style={{ padding: 0 }}
            disabled={selected.length === projects.length}
            onClick={() => {
              const ids = projects.map(x => x.id);
              setSelected(ids);
              setPermByProject(prev => {
                const next = { ...prev };
                for (const id of ids) if (!(id in next)) next[id] = PERM.projectView;
                return next;
              });
            }}
          >
            Выбрать все проекты
          </Button>
          {selected.length > 0 && (
            <Button
              size="small"
              type="link"
              style={{ padding: 0, marginLeft: 16 }}
              onClick={() => { setSelected([]); setPermByProject({}); }}
            >
              Снять все
            </Button>
          )}
        </div>

        {selected.length > 0 && (
          <div style={{ marginTop: 12, border: `1px solid ${LINE}`, borderRadius: 8 }}>
            {selected.map((id, i) => {
              const p = projects.find(x => x.id === id);
              return (
                <div
                  key={id}
                  style={{
                    display: 'flex', alignItems: 'center', justifyContent: 'space-between',
                    gap: 12, padding: '8px 12px',
                    borderTop: i === 0 ? 'none' : `1px solid ${LINE}`,
                  }}
                >
                  <span style={{
                    minWidth: 0, flex: 1, overflow: 'hidden', textOverflow: 'ellipsis',
                    whiteSpace: 'nowrap',
                  }}>
                    {p ? `${p.id} · ${p.name}` : `Проект №${id}`}
                  </span>
                  <Select
                    size="small"
                    value={permByProject[id] ?? PERM.projectView}
                    onChange={(v) => setPermByProject(s => ({ ...s, [id]: v }))}
                    options={PROJECT_PERMISSION_OPTIONS}
                    style={{ width: 220, flexShrink: 0 }}
                  />
                </div>
              );
            })}
          </div>
        )}

        <Button
          style={{ marginTop: 12 }}
          onClick={() => saveAccessMutation.mutate()}
          loading={saveAccessMutation.isPending}
        >
          Сохранить доступ
        </Button>
        </>
        )}
      </Section>

      <Section title="Пароль" last>
        <Form form={pwdForm} layout="vertical" onFinish={pwdMutation.mutate}>
          <Form.Item
            name="password"
            label="Новый пароль"
            rules={[{ required: true, min: 10, message: 'Минимум 10 символов' }]}
          >
            <Input.Password placeholder="Не менее 10 символов" />
          </Form.Item>
          <Button htmlType="submit" loading={pwdMutation.isPending}>
            Сменить пароль
          </Button>
        </Form>
      </Section>
    </Drawer>
  );
}

function Section({ title, children, last }: {
  title: string; children: React.ReactNode; last?: boolean;
}) {
  return (
    <div style={{
      paddingBottom: 18,
      marginBottom: 18,
      borderBottom: last ? 'none' : `1px solid ${LINE}`,
    }}>
      <div style={{
        fontSize: 12, letterSpacing: 0.4, textTransform: 'uppercase',
        color: TEXT_SOFT, marginBottom: 10,
      }}>
        {title}
      </div>
      {children}
    </div>
  );
}
