import { useQuery } from '@tanstack/react-query';
import { profileApi } from '../api';

// Права текущего пользователя на фронте. Права считает СЕРВЕР — здесь только
// копия его ответа, по которой прячутся недоступные кнопки и разделы.

export const PERM = {
  projectView: 'project.view',
  projectCreate: 'project.create',
  projectEdit: 'project.edit',
  projectStatus: 'project.status',
  budgetView: 'budget.view',
  budgetCreate: 'budget.create',
  budgetVersion: 'budget.version',
  budgetEdit: 'budget.edit',
  budgetStatus: 'budget.status',
  budgetApprove: 'budget.approve',
  budgetExport: 'budget.export',
  referencesEdit: 'references.edit',
  auditView: 'audit.view',
  usersManage: 'users.manage',
} as const;

export type Permission = (typeof PERM)[keyof typeof PERM];

export const PERMISSION_LABELS: Record<string, string> = {
  'project.view': 'Просмотр проекта',
  'project.create': 'Создание карточки проекта',
  'project.edit': 'Изменение карточки проекта',
  'project.status': 'Смена статуса проекта',
  'budget.view': 'Просмотр бюджета',
  'budget.create': 'Создание первой версии бюджета',
  'budget.version': 'Создание новой версии бюджета',
  'budget.edit': 'Изменение версии бюджета',
  'budget.status': 'Смена статуса бюджета',
  'budget.approve': 'Согласование бюджета',
  'budget.export': 'Выгрузка бюджета в xlsx',
  'references.edit': 'Изменение справочников',
  'audit.view': 'Просмотр истории изменений',
  'users.manage': 'Управление пользователями',
};

/** Псевдоправо «все права» — тумблер «все права на один проект». */
export const PERM_ALL = '*';

// Права, которые главный экономист выдаёт индивидуально. «Управление
// пользователями» в списке нет: выдав его, ГЭ получил бы второго ГЭ — для
// этого есть смена роли.
export const GRANTABLE: string[] = [
  PERM.projectView, PERM.projectCreate, PERM.projectEdit, PERM.projectStatus,
  PERM.budgetView, PERM.budgetCreate, PERM.budgetVersion, PERM.budgetEdit,
  PERM.budgetStatus, PERM.budgetApprove, PERM.budgetExport,
  PERM.referencesEdit, PERM.auditView,
];

/** Права, которые действуют глобально и к проекту не привязываются. */
const GLOBAL_ONLY = new Set<string>([
  PERM.projectCreate, PERM.referencesEdit, PERM.usersManage,
]);

export function isProjectScoped(perm: string): boolean {
  return !GLOBAL_ONLY.has(perm);
}

// Опции выпадающего списка «право на проекте» в панели пользователя
// (`UserPanel`, блок «Доступ к проектам»): один список проектов, справа от
// каждого — одно из этих прав, вместо двух отдельных панелей (тумблер
// «правка/просмотр» и отдельная выдача индивидуальных прав).
export const PROJECT_PERMISSION_OPTIONS: { value: string; label: string }[] = [
  { value: PERM_ALL, label: 'Все права' },
  { value: PERM.projectView, label: 'Просмотр проекта' },
  ...GRANTABLE
    .filter((p) => p !== PERM.projectView && isProjectScoped(p))
    .map((p) => ({ value: p, label: PERMISSION_LABELS[p] })),
];

// Главный экономист — единственная роль, у которой все 14 прав действуют на
// всех проектах (roleMatrix[RoleGE], все клетки ScopeAll).
export function hasFullAccessByRole(role: string): boolean {
  return role === 'GE';
}

/**
 * Права текущего пользователя. Берёт их из того же запроса профиля,
 * который уже делает шапка, — лишнего обращения к серверу нет.
 */
export function usePermissions() {
  const { data, isLoading } = useQuery({
    queryKey: ['profile'],
    queryFn: profileApi.get,
  });
  const list = data?.permissions ?? [];
  return {
    loading: isLoading,
    permissions: list,
    /** Может ли пользователь это действие хотя бы где-нибудь. */
    can: (perm: string) => list.includes(perm),
  };
}

/**
 * Может ли действие быть выполнено в конкретном проекте.
 * Список прав приходит вместе с карточкой проекта и версией бюджета.
 */
export function canIn(permissions: string[] | undefined, perm: string): boolean {
  return (permissions ?? []).includes(perm);
}
