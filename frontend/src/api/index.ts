import { client } from './client';
import type {
  AuthResponse, User,
  Project, ProjectListItem,
  BudgetVersion, CalcResult,
  Executor, Position, WorkMode, CostItem,
  AuditEntry, PaginatedResponse, Profile,
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

  update: (data: { avatar?: string; notes?: string }) =>
    client.put<Profile>('/users/me', data).then(r => r.data),

  changePassword: (currentPassword: string, newPassword: string) =>
    client.put<{ status: string }>('/users/me/password', {
      current_password: currentPassword,
      new_password: newPassword,
    }).then(r => r.data),
};

// ─── Users ─────────────────────────────────────────────────────────────────
export const usersApi = {
  list: () =>
    client.get<User[]>('/users').then(r => r.data),

  get: (id: number) =>
    client.get<User>(`/users/${id}`).then(r => r.data),

  create: (data: { email: string; full_name: string; role: string; password: string }) =>
    client.post<User>('/users', data).then(r => r.data),

  update: (id: number, data: { full_name?: string; role?: string; active?: boolean }) =>
    client.put<User>(`/users/${id}`, data).then(r => r.data),

  setPassword: (id: number, password: string) =>
    client.post(`/users/${id}/password`, { password }),

  unlock: (id: number) =>
    client.post(`/users/${id}/unlock`),

  grantAccess: (userId: number, projectId: number, canEdit = false) =>
    client.post(`/users/${userId}/projects/${projectId}/grant`, { can_edit: canEdit }),

  revokeAccess: (userId: number, projectId: number) =>
    client.delete(`/users/${userId}/projects/${projectId}/access`),
};

// ─── References ────────────────────────────────────────────────────────────
export const refsApi = {
  executors: () => client.get<Executor[]>('/references/executors').then(r => r.data),
  positions: () => client.get<Position[]>('/references/positions').then(r => r.data),
  workModes: () => client.get<WorkMode[]>('/references/work-modes').then(r => r.data),
  costItems: () => client.get<CostItem[]>('/references/cost-items').then(r => r.data),

  createExecutor: (data: { name: string; full_name: string }) =>
    client.post<Executor>('/references/executors', data).then(r => r.data),
  updateExecutor: (id: number, data: { name?: string; full_name?: string; active?: boolean }) =>
    client.put<Executor>(`/references/executors/${id}`, data).then(r => r.data),

  createPosition: (name: string) =>
    client.post<Position>('/references/positions', { name }).then(r => r.data),
  updatePosition: (id: number, data: { name?: string; active?: boolean }) =>
    client.put<Position>(`/references/positions/${id}`, data).then(r => r.data),

  createWorkMode: (data: { code: string; full_name: string }) =>
    client.post<WorkMode>('/references/work-modes', data).then(r => r.data),
  updateWorkMode: (id: number, data: { full_name?: string; active?: boolean }) =>
    client.put<WorkMode>(`/references/work-modes/${id}`, data).then(r => r.data),
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
};

// ─── Audit ─────────────────────────────────────────────────────────────────
export const auditApi = {
  list: (params?: { page?: number; limit?: number; offset?: number; action?: string; user_id?: number }) =>
    client.get<{ total: number; items: AuditEntry[] }>('/audit-log', { params }).then(r => r.data),
};
