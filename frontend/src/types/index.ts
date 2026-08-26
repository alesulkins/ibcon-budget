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
  /** Порядковый номер версии внутри проекта (1, 2, 3…). */
  version_no: number;
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
  /**
   * Номера месяцев проекта (1-based), в которых начисляется уборка.
   * Пустой список — уборки нет за весь период.
   */
  cleaning_months: number[];
}

/**
 * Одна разовая покупка: строка таблицы 4.3!A17:D27 (авто) или
 * 4.4!A12:D22 (вагончики). Итог строки = цена × кол-во.
 */
export interface ItemPurchase {
  name: string;
  /** Месяц покупки, 1-based. 0 — строка не заполнена, в расчёт не идёт. */
  month: number;
  /** Количество (4.3!B17:B27, 4.4!B12:B22). */
  count: number;
  /** Стоимость одной единицы (4.3!C17:C27, 4.4!C12:C22). */
  price: number;
}

/** Одна строка аренды: авто (4.3 строки 5-6) или гараж (4.3 строки 10-11). */
export interface RentedItem {
  name: string;
  /** Цена аренды одной единицы за месяц (4.3!B6 / 4.3!B11). */
  price: number;
  /**
   * Количество единиц по месяцам, индекс 0 = первый месяц проекта
   * (4.3!C5:BJ5 для авто, 4.3!C10:BJ10 для гаража). 0 — не арендуем.
   */
  counts: number[];
}

/**
 * Транспорт (лист 4.3). Суммы НЕ передаются: их считает calcTransport
 * на бэкенде. Покупка авто и аренда авто уходят одной строкой бюджета
 * («Аренда транспорта»), гараж — отдельной.
 */
export interface InputTransport {
  car_purchases: ItemPurchase[];
  car_rentals: RentedItem[];
  garage_rentals: RentedItem[];
}

/** Цена за единицу плюс количество в каждом месяце (лист 4.4). */
export interface MonthlyQty {
  price: number;
  /** Количество по месяцам, индекс 0 = первый месяц проекта. */
  counts: number[];
}

/**
 * Вагончики (лист 4.4). Аренда и покупка складываются в одну строку
 * бюджета «Обустройство строительной площадки»; считает calcWagonciks.
 */
export interface InputWagonciks {
  rental: MonthlyQty;
  purchases: ItemPurchase[];
}

/**
 * Офис (лист 4.5). Суммы НЕ передаются: их считает calcOffice.
 * Аренда (4.5!C5) и уборка (4.5!C11) уходят в ДВЕ разные строки бюджета.
 * Уборка = количество офисов в месяце × cleaning_price (4.5!C19*$B$11).
 */
export interface InputOffice {
  /** Арендуемые офисы: цена за месяц + количество по месяцам (4.5!B15:B17, C15:BJ17). */
  offices: RentedItem[];
  /** Стоимость уборки ОДНОГО офиса за месяц (4.5!B11). */
  cleaning_price: number;
}

/**
 * Сколько первых месяцев проекта открыты для покупки вагончиков.
 * Покупка невозможна в последние два месяца; проект в 1 месяц —
 * оговорённое исключение, там покупка разрешена.
 * Дублирует purchaseAllowedMonths из internal/calc/wagonciks.go.
 */
export function purchaseAllowedMonths(duration: number): number {
  if (duration === 1) return 1;
  if (duration < 2) return 0;
  return duration - 2;
}

/**
 * Одна позиция листа-списка: наименование и стоимость по месяцам.
 * Общая структура для 4.8 (ПО), 4.9 (ГПХ внешний) и 4.11 (субподряд) —
 * листы устроены одинаково, поэтому и форма у них одна.
 */
export interface CostLine {
  name: string;
  /**
   * Стоимость по месяцам проекта, индекс 0 = первый месяц. Длина равна
   * длительности проекта; пусто или 0 — в этом месяце позиции нет.
   */
  monthly_amounts: number[];
}

/** Ввод листа-списка (4.8, 4.9, 4.11). Итог месяца = сумма всех позиций. */
export interface InputCostLines {
  lines: CostLine[];
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
  /**
   * ТКП — стоимость договора (2.Бюджет!G251). Единственный ручной ввод
   * блока выручки. Отдельного поля «ручная стоимость работ» больше нет:
   * в форме это формула F236 = G252 = ТКП без НДС, бэкенд выводит её сам.
   */
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
  user_name: string;
  user_role: string;
  action: string;
  object_type: string;
  object_id?: number;
  comment: string;
  /** Проект записи: для действий над версией бюджета — через её бюджет. */
  project_id?: number;
  project_name: string;
}

// ─── API generics ───────────────────────────────────────────────────────────
export interface PaginatedResponse<T> {
  items: T[];
  total: number;
  page: number;
  limit: number;
}

/** Данные личного кабинета (GET /users/me). */
export interface Profile {
  id: number;
  email: string;
  /** Полное ФИО. Сокращённую форму даёт shortName() из utils/names. */
  full_name: string;
  role: string;
  /** Эмодзи или data:-URL загруженной картинки. Пусто — показываем инициалы. */
  avatar: string;
  notes: string;
}
