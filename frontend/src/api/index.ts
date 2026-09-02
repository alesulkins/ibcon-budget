import { client } from './client';
import type {
  AuthResponse, User,
  Project, ProjectListItem,
  BudgetVersion, CalcResult, BudgetReport,
  Executor, Position, WorkMode, CostItem, City, CitySalary,
  AuditEntry, PaginatedResponse, Profile, Reminder, UISettings,
  RentMarketQuery, RentMarketEstimate,
} from '../types';

// ─── Auth ──────────────────────────────────────────────────────────────────
export const authApi = {
  login: (email: string, password: string, rememberMe = false) =>
    client
      .post<AuthResponse>('/auth/login', { email, password, remember_me: rememberMe })
      .then(r => r.data),
};

// ─── Личный кабинет ────────────────────────────────────────────────────────
export const profileApi = {
  get: () => client.get<Profile>('/users/me').then(r => r.data),

  update: (data: { avatar?: string; notes?: string; ui_settings?: UISettings }) =>
    client.put<Profile>('/users/me', data).then(r => r.data),

  changePassword: (currentPassword: string, newPassword: string) =>
    client.put<{ status: string }>('/users/me/password', {
      current_password: currentPassword,
      new_password: newPassword,
    }).then(r => r.data),
};

// ─── Рыночная стоимость аренды ─────────────────────────────────────────────
// Запрос идёт к площадкам объявлений и занимает секунды, поэтому это
// POST по требованию, а не фоновая загрузка страницы.
export const marketApi = {
  rentEstimate: (q: RentMarketQuery) =>
    client.post<RentMarketEstimate>('/market/rent-estimate', q).then(r => r.data),
};

// ─── Напоминания ───────────────────────────────────────────────────────────
// Напоминания личные: id владельца сервер берёт из токена, в запросах его
// нет вовсе.
export const remindersApi = {
  list: () => client.get<Reminder[]>('/reminders').then(r => r.data),

  create: (text: string, remindAt: string) =>
    client.post<Reminder>('/reminders', { text, remind_at: remindAt }).then(r => r.data),

  update: (id: number, data: { text?: string; remind_at?: string; done?: boolean }) =>
    client.put<Reminder>(`/reminders/${id}`, data).then(r => r.data),

  remove: (id: number) => client.delete(`/reminders/${id}`),

  /** Наступившие и ещё не показанные. */
  due: () => client.get<Reminder[]>('/reminders/due').then(r => r.data),

  /** Подтвердить показ, чтобы уведомление не всплывало заново. */
  markShown: (ids: number[]) => client.post('/reminders/shown', { ids }),
};

// ─── Users ─────────────────────────────────────────────────────────────────
export const usersApi = {
  list: () =>
    client.get<User[]>('/users').then(r => r.data),

  get: (id: number) =>
    client.get<User>(`/users/${id}`).then(r => r.data),

  create: (data: { email: string; full_name: string; role: string; password: string }) =>
    client.post<User>('/users', data).then(r => r.data),

  update: (id: number, data: { full_name?: string; email?: string; role?: string; active?: boolean }) =>
    client.put<User>(`/users/${id}`, data).then(r => r.data),

  setPassword: (id: number, password: string) =>
    client.post(`/users/${id}/password`, { password }),

  unlock: (id: number) =>
    client.post(`/users/${id}/unlock`),

  grantAccess: (userId: number, projectId: number, canEdit = false) =>
    client.post(`/users/${userId}/projects/${projectId}/grant`, { can_edit: canEdit }),

  revokeAccess: (userId: number, projectId: number) =>
    client.delete(`/users/${userId}/projects/${projectId}/access`),

  /**
   * Приводит доступ пользователя к переданному списку проектов:
   * чего в списке нет — отзывается. Вместе с отозванным проектом
   * снимаются и выданные на него индивидуальные права.
   */
  setProjects: (userId: number, projects: { project_id: number; can_edit: boolean }[]) =>
    client.put(`/users/${userId}/projects`, { projects }),

  /** Выдать индивидуальное право. project_id === null — на все проекты. */
  grantPermission: (userId: number, permission: string, projectId: number | null) =>
    client.post(`/users/${userId}/permissions`, { permission, project_id: projectId }),

  revokePermission: (userId: number, permission: string, projectId: number | null) =>
    client.delete(`/users/${userId}/permissions`, {
      data: { permission, project_id: projectId },
    }),
};

// ─── References ────────────────────────────────────────────────────────────
/**
 * Списки отдают только активные записи — их читают формы мастера, и
 * выбрать деактивированную запись там нельзя. Экран управления
 * справочниками показывает всё и просит это явно: all = true.
 */
const allParams = (all?: boolean) => (all ? { params: { all: true } } : undefined);

export const refsApi = {
  executors: (all?: boolean) =>
    client.get<Executor[]>('/references/executors', allParams(all)).then(r => r.data),
  positions: (all?: boolean) =>
    client.get<Position[]>('/references/positions', allParams(all)).then(r => r.data),
  workModes: (all?: boolean) =>
    client.get<WorkMode[]>('/references/work-modes', allParams(all)).then(r => r.data),
  costItems: (all?: boolean) =>
    client.get<CostItem[]>('/references/cost-items', allParams(all)).then(r => r.data),

  createExecutor: (data: {
    name: string; full_name: string;
    profit_tax_rate?: number; refinancing_rate?: number;
  }) => client.post<Executor>('/references/executors', data).then(r => r.data),
  updateExecutor: (id: number, data: {
    name?: string; full_name?: string;
    profit_tax_rate?: number; refinancing_rate?: number; active?: boolean;
  }) => client.put<Executor>(`/references/executors/${id}`, data).then(r => r.data),

  createPosition: (data: { name: string; salary?: number; is_itr?: boolean }) =>
    client.post<Position>('/references/positions', data).then(r => r.data),
  updatePosition: (id: number, data: { name?: string; salary?: number; is_itr?: boolean; active?: boolean }) =>
    client.put<Position>(`/references/positions/${id}`, data).then(r => r.data),

  createWorkMode: (data: { code: string; full_name: string }) =>
    client.post<WorkMode>('/references/work-modes', data).then(r => r.data),
  updateWorkMode: (id: number, data: { full_name?: string; active?: boolean }) =>
    client.put<WorkMode>(`/references/work-modes/${id}`, data).then(r => r.data),

  /** Города для окладов должности. Пополняются прямо в форме должности. */
  cities: (all?: boolean) =>
    client.get<City[]>('/references/cities', allParams(all)).then(r => r.data),
  createCity: (name: string) =>
    client.post<City>('/references/cities', { name }).then(r => r.data),

  /**
   * Заменяет оклады должности по городам целиком: город, пропавший из
   * списка, теряет ставку — иначе снятую строку нечем было бы удалить.
   */
  setCitySalaries: (positionId: number, salaries: CitySalary[]) =>
    client.put<Position>(`/references/positions/${positionId}/salaries`, { salaries })
      .then(r => r.data),

  createCostItem: (data: { name: string }) =>
    client.post<CostItem>('/references/cost-items', data).then(r => r.data),
  updateCostItem: (id: number, data: { name?: string; active?: boolean }) =>
    client.put<CostItem>(`/references/cost-items/${id}`, data).then(r => r.data),
};

// ─── Projects ──────────────────────────────────────────────────────────────
export const projectsApi = {
  list: (params?: { page?: number; limit?: number; search?: string; status?: string }) =>
    client.get<PaginatedResponse<ProjectListItem>>('/projects', { params }).then(r => r.data),

  get: (id: number) =>
    client.get<Project>(`/projects/${id}`).then(r => r.data),

  create: (data: {
    name: string; customer: string; executor_id: number; location: string;
    start_date: string; duration_months: number; director: string;
    manager: string; administrator: string; economist: string; status: string;
  }) => client.post<Project>('/projects', data).then(r => r.data),

  update: (id: number, data: Partial<{
    name: string; customer: string; executor_id: number; location: string;
    start_date: string; duration_months: number; director: string;
    manager: string; administrator: string; economist: string;
  }>) => client.put<Project>(`/projects/${id}`, data).then(r => r.data),

  changeStatus: (id: number, status: string, comment: string) =>
    client.patch<Project>(`/projects/${id}/status`, { status, comment }).then(r => r.data),
};

// ─── Budgets ───────────────────────────────────────────────────────────────
export const budgetsApi = {
  listVersions: (projectId: number) =>
    client.get<BudgetVersion[]>(`/projects/${projectId}/budgets/versions`).then(r => r.data),

  createVersion: (projectId: number, data: { copy_from_id?: number; comment?: string }) =>
    client.post<BudgetVersion>(`/projects/${projectId}/budgets/versions`, data).then(r => r.data),

  newVersion: (projectId: number, data: { copy_from_id?: number; comment?: string }) =>
    client.post<BudgetVersion>(`/projects/${projectId}/budgets/new-version`, data).then(r => r.data),

  getVersion: (vid: number) =>
    client.get<BudgetVersion>(`/budget-versions/${vid}`).then(r => r.data),

  changeStatus: (vid: number, status: string, comment: string) =>
    client.patch<BudgetVersion>(`/budget-versions/${vid}/status`, { status, comment }).then(r => r.data),

  updateCostOverride: (vid: number, cost_override: number | null) =>
    client.put<BudgetVersion>(`/budget-versions/${vid}/cost-override`, { cost_override }).then(r => r.data),

  saveInput: (vid: number, type: string, data: unknown) =>
    client.put(`/budget-versions/${vid}/inputs/${type}`, data),

  getInput: <T>(vid: number, type: string) =>
    client.get<T>(`/budget-versions/${vid}/inputs/${type}`).then(r => r.data),

  getAllInputs: (vid: number) =>
    client.get<Record<string, unknown>>(`/budget-versions/${vid}/inputs`).then(r => r.data),

  calculate: (vid: number) =>
    client.get<CalcResult>(`/budget-versions/${vid}/calculate`).then(r => r.data),

  /**
   * Выгружает версию бюджета в xlsx и отдаёт файл браузеру. Книга
   * содержит три листа сразу — «Бюджет», БДР и БДДС; у администратора
   * проекта только один урезанный лист «Бюджет».
   *
   * Имя файла берём из Content-Disposition: сервер кладёт его туда в
   * filename* с кодировкой UTF-8, иначе русское название проекта
   * сохранилось бы крякозябрами.
   */
  exportXlsx: (vid: number) =>
    downloadFile(`/budget-versions/${vid}/export`, `budget-${vid}.xlsx`),

  /** БДР и БДДС одной выборкой: на экране это соседние вкладки. */
  reports: (vid: number) =>
    client.get<{ bdr: BudgetReport; bdds: BudgetReport }>(
      `/budget-versions/${vid}/reports`,
    ).then(r => r.data),
};

/**
 * Скачивание файла с сервера. Имя берём из Content-Disposition —
 * сервер кладёт его в filename* с кодировкой UTF-8, иначе русское
 * название проекта сохранилось бы крякозябрами.
 */
async function downloadFile(url: string, fallbackName: string) {
  const res = await client.get(url, { responseType: 'blob' });

  const disposition = String(res.headers['content-disposition'] ?? '');
  const utf8 = /filename\*=UTF-8''([^;]+)/i.exec(disposition);
  const name = utf8 ? decodeURIComponent(utf8[1]) : fallbackName;

  const href = URL.createObjectURL(res.data as Blob);
  const a = document.createElement('a');
  a.href = href;
  a.download = name;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(href);
  return name;
}

// ─── Audit ─────────────────────────────────────────────────────────────────
export const auditApi = {
  list: (params?: { page?: number; limit?: number; offset?: number; action?: string; user_id?: number }) =>
    client.get<{ total: number; items: AuditEntry[] }>('/audit-log', { params }).then(r => r.data),
};
