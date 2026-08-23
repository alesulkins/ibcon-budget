// ─── Auth ──────────────────────────────────────────────────────────────────
export interface User {
  id: number;
  email: string;
  full_name: string;
  role: string;
  active: boolean;
  failed_attempts: number;
  locked_until?: string;
  created_at: string;
  created_by?: number;
}

export interface AuthResponse {
  token: string;
  user_id: number;
  full_name: string;
  role: string;
}

export interface Claims {
  user_id: number;
  email: string;
  role: string;
  full_name: string;
}

// Роли
export const ROLES = {
  GE: 'GE',           // Главный экономист
  EP: 'EP',           // Экономист проекта
  IP: 'IP',           // Инициатор проекта
  RP: 'RP',           // Руководитель проекта
  AP: 'AP',           // Администратор проекта
  MANAGEMENT: 'MANAGEMENT',
} as const;

export const ROLE_LABELS: Record<string, string> = {
  GE: 'Главный экономист',
  EP: 'Экономист проекта',
  IP: 'Инициатор проекта',
  RP: 'Руководитель проекта',
  AP: 'Администратор проекта',
  MANAGEMENT: 'Руководство',
};

// ─── References ────────────────────────────────────────────────────────────
export interface Executor {
  id: number;
  name: string;
  full_name: string;
  active: boolean;
}

export interface Position {
  id: number;
  name: string;
  active: boolean;
}

export interface WorkMode {
  id: number;
  code: string;
  full_name: string;
  active: boolean;
}

export interface CostItem {
  id: number;
  name: string;
  is_calculated: boolean;
  active: boolean;
}

// ─── Projects ──────────────────────────────────────────────────────────────
export const PROJECT_STATUSES = {
  PROSPECT: 'prospect',
  ACTIVE: 'active',
  SUSPENDED: 'suspended',
  COMPLETED: 'completed',
  UNREALIZED: 'unrealized',
} as const;

export const PROJECT_STATUS_LABELS: Record<string, string> = {
  prospect: 'Перспективный',
  active: 'Действующий',
  suspended: 'Приостановлен',
  completed: 'Завершён',
  unrealized: 'Не реализован',
};

export const PROJECT_STATUS_COLORS: Record<string, string> = {
  prospect: 'blue',
  active: 'green',
  suspended: 'orange',
  completed: 'default',
  unrealized: 'red',
};

export interface Project {
  id: number;
  name: string;
  customer: string;
  executor_id: number;
  executor_name: string;
  location: string;
  start_date: string;
  duration_months: number;
  end_date: string;
  director: string;
  manager: string;
  administrator: string;
  economist: string;
  status: string;
  created_at: string;
  created_by: number;
  created_by_name: string;
  updated_at: string;
}

export interface ProjectListItem {
  id: number;
  name: string;
  customer: string;
  executor_name: string;
  director: string;
  manager: string;
  administrator: string;
  economist: string;
  status: string;
  budget_status?: string;
  cost_no_vat?: number;
  profitability?: number;
  created_at: string;
  created_by_name: string;
}

// ─── Budgets ────────────────────────────────────────────────────────────────
export const BUDGET_STATUSES = {
  DRAFT: 'draft',
  UNDER_REVIEW: 'under_review',
  APPROVED: 'approved',
  ARCHIVE: 'archive',
} as const;

export const BUDGET_STATUS_LABELS: Record<string, string> = {
  draft: 'Черновик',
  under_review: 'На согласовании',
  approved: 'Согласован',
  archive: 'Архив',
};

export const BUDGET_STATUS_COLORS: Record<string, string> = {
  draft: 'default',
  under_review: 'orange',
  approved: 'green',
  archive: 'default',
};

export interface BudgetVersion {
  id: number;
  budget_id: number;
  project_id: number;
  version_label?: string;
  status: string;
  comment?: string;
  cost_no_vat?: number;
  profitability?: number;
  cost_override?: number;
  created_at: string;
  created_by: number;
  created_by_name: string;
  updated_at: string;
  approved_at?: string;
  copied_from?: number;
}

// ─── Calc types ──────────────────────────────────────────────────────────────
export interface Employee {
  position: string;
  full_name: string;
  country: string;
  base_schedule: string;
  salary_net: number;
  monthly_schedule: string[];
  trip_days_rf: number[];
  trip_days_other: number[];
}

/** Виды премий (лист 4.1). Значения совпадают с константами бэкенда. */
export const BONUS_KIND_BUILDER_DAY = 'день_строителя';
export const BONUS_KIND_NEW_YEAR = 'новый_год';
export const BONUS_KIND_OTHER = 'другое';

export interface BonusType {
  /** день_строителя / новый_год / другое */
  kind: string;
  name: string;
  /** Календарный месяц начисления, 1-12. 0 → дефолт по kind. */
  month_num: number;
  /** Процент от оклада числом: 20 означает 20%. 0 → дефолт по kind. */
  pct_of_salary: number;
}

/**
 * Премии (лист 4.1). Суммы НЕ передаются: их считает бэкенд
 * (calcBonuses) из видов премий и списка сотрудников.
 */
export interface InputBonuses {
  bonus_types: BonusType[];
}

/**
 * Аренда квартир (лист 4.2). Передаём количество квартир по типам и
 * цены, а НЕ готовую сумму: аренду, уборку и риелтора считает
 * calcRentApartments на бэкенде.
 */
export interface InputRentApartments {
  price_1room: number;
  price_2room: number;
  price_3room: number;
  count_1room: number[];
  count_2room: number[];
  count_3room: number[];
  /** Базовая стоимость уборки одной квартиры за месяц (4.2!B9) */
  cleaning_base: number;
  /** Базовая стоимость услуг риелтора за одну квартиру (4.2!B13) */
  realtor_base: number;
}

export interface InputEmployees {
  ticket_price: number;
  per_diem_rf: number;
  per_diem_other: number;
  employees: Employee[];
}

export interface BankGuarantee {
  pct: number;
  rate_pct: number;
  rate_type: string;
  duration_mos: number;
}

export interface InputBudgetParams {
  unpredictables_pct: number;
  aup_pct: number;
  other_expense_mode: string;
  other_expense_value: number;
  bg_execution: BankGuarantee;
  bg_warranty: BankGuarantee;
  bg_advance: BankGuarantee;
  /**
   * Целевая рентабельность без налога на прибыль, % (2.Бюджет!E234).
   * Коэффициент наценки бэкенд считает сам: E234/(1-F240-E234).
   */
  target_rent_pct: number;
  manual_revenue: number;
  contract_value: number;
}

export interface MonthlyResult {
  month: number;
  fot: number;
  bonuses: number;
  overtime_rf: number;
  overtime_kg: number;
  ndfl: number;
  insurance_rf: number;
  insurance_kg: number;
  total_fot: number;
  overhead: number[];
  tickets: number;
  per_diem: number;
  project_costs_ex_fot: number;
  unpredictables: number;
  aup: number;
  total_costs_gross: number;
  other_expenses: number;
  bg_execution: number;
  bg_warranty: number;
  bg_advance: number;
  total_costs: number;
  margin_amount: number;
  revenue: number;
  operating_profit: number;
  revenue_with_vat: number;
}

export interface CalcResult {
  duration_months: number;
  monthly: MonthlyResult[];
  total_fot: number;
  total_costs: number;
  total_revenue: number;
  operating_profit: number;
  tax: number;
  net_profit: number;
  profitability: number;
  total_revenue_with_vat: number;
}

// ─── Audit ──────────────────────────────────────────────────────────────────
export interface AuditEntry {
  id: number;
  occurred_at: string;
  user_id?: number;
  user_role: string;
  action: string;
  object_type: string;
  object_id?: number;
  comment: string;
}

// ─── API generics ───────────────────────────────────────────────────────────
export interface PaginatedResponse<T> {
  items: T[];
  total: number;
  page: number;
  limit: number;
}
