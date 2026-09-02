// ─── Auth ──────────────────────────────────────────────────────────────────
export interface User {
  id: number;
  email: string;
  /** Полное ФИО. В таблицах показывается как «Фамилия И.О.». */
  full_name: string;
  /** Пусто — роль не назначена: вход есть, функциональности нет. */
  role: string;
  active: boolean;
  failed_attempts: number;
  locked_until?: string;
  created_at: string;
  created_by?: number;
  /** Проекты, к которым выдан доступ (приходит в списке пользователей). */
  projects?: ProjectAccess[];
  /** Индивидуальные права сверх роли. */
  grants?: Grant[];
}

/** Назначение пользователя на проект. */
export interface ProjectAccess {
  user_id: number;
  project_id: number;
  name: string;
  status: string;
  /** false — только просмотр. */
  can_edit: boolean;
}

/** Индивидуальное право сверх роли. project_id null — на все проекты. */
export interface Grant {
  permission: string;
  project_id: number | null;
  project_name: string | null;
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
  '': 'Роль не назначена',
  GE: 'Главный экономист',
  EP: 'Экономист проекта',
  IP: 'Инициатор проекта',
  RP: 'Руководитель проекта',
  AP: 'Администратор проекта',
  MANAGEMENT: 'Руководство',
};

// ─── References ────────────────────────────────────────────────────────────
/**
 * Общее для всех справочников: записи не удаляются, только
 * деактивируются; кто и когда менял запись — updated_at/updated_by_name.
 * Уже сохранённые бюджеты правку справочника не замечают: они хранят
 * подставленное значение, а не ссылку.
 */
interface ReferenceRow {
  id: number;
  active: boolean;
  updated_at: string;
  updated_by_name?: string;
}

export interface Executor extends ReferenceRow {
  name: string;
  full_name: string;
  /** Справочные значения в процентах: 25 = 25%. */
  profit_tax_rate: number;
  refinancing_rate: number;
}

export interface Position extends ReferenceRow {
  name: string;
  /**
   * Оклад ПО УМОЛЧАНИЮ: применяется там, где для города проекта своей
   * ставки не задали.
   */
  salary: number;
  /**
   * Инженерно-технический работник. По этому признаку в выгрузке
   * бюджета считается сводка по ИТР; рабочие и администрация в неё не
   * входят.
   */
  is_itr: boolean;
  /**
   * Оклад по городам: в разных городах за одну работу платят по-разному,
   * и в мастер подставляется ставка города проекта.
   */
  city_salaries?: CitySalary[];
}

/** Город из справочника городов. Список пополняется в форме должности. */
export interface City {
  id: number;
  name: string;
  active: boolean;
  updated_at: string;
}

/** Оклад должности в конкретном городе. */
export interface CitySalary {
  city_id: number;
  city_name: string;
  salary: number;
}

/**
 * Оклад должности в городе проекта. Города сравниваем по имени и
 * регистронезависимо: в карточке проекта город записан строкой, а не
 * ссылкой на справочник. Своей ставки нет — берётся оклад по умолчанию.
 *
 * Тот же порядок независимо повторяет бэкенд (references.SalaryFor):
 * там он нужен на случай, если оклад подставляется не из формы.
 */
export function salaryForCity(position: Position, location: string): number {
  const key = location.trim().toLowerCase();
  const hit = (position.city_salaries ?? [])
    .find(cs => cs.city_name.trim().toLowerCase() === key);
  return hit ? hit.salary : position.salary;
}

export interface WorkMode extends ReferenceRow {
  code: string;
  full_name: string;
}

export interface CostItem extends ReferenceRow {
  name: string;
  /** Расчётную статью нельзя ни добавить, ни изменить, ни деактивировать. */
  is_calculated: boolean;
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

/** Ключи палитры STATUS из theme.ts — приглушённые, не палитра antd. */
export const PROJECT_STATUS_COLORS: Record<string, string> = {
  prospect: 'blue',
  active: 'green',
  suspended: 'orange',
  completed: 'grey',
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
  /**
   * Что текущий пользователь может делать с ЭТИМ проектом. Считает
   * сервер; фронт по списку прячет кнопки, запрет обеспечивает бэкенд.
   */
  permissions?: string[];
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

/** Ключи палитры STATUS из theme.ts. */
export const BUDGET_STATUS_COLORS: Record<string, string> = {
  draft: 'grey',
  under_review: 'amber',
  approved: 'green',
  archive: 'grey',
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
  /** Права текущего пользователя на бюджеты этого проекта. */
  permissions?: string[];
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
  /**
   * Множитель графика (4.6!BU16), заданный руками по месяцам.
   * `null` в ячейке — значение считается автоматически (это и есть флаг
   * «не трогали»); число — ручное значение, оно перекрывает формулу и
   * идёт в расчёт ФОТ. Ноль — законное ручное значение, отличать его от
   * `null` обязательно. Массив может быть короче графика.
   */
  multiplier_overrides?: (number | null)[];
}

/** Фиксированная выплата за межвахтовый отдых («МВ»), 4.6!BU16. */
export const INTER_SHIFT_PAY = 30_000;

/**
 * Автоматический множитель графика — та же формула, что в
 * scheduleMultiplier из internal/calc/fot.go:
 *
 *   =ЕСЛИ(график="МВ"; 30000/оклад; ЕСЛИ(график="не принят"; 0; 1))
 *
 * Здесь она нужна, чтобы показать значение в таблице до сохранения;
 * в расчёт идёт значение, посчитанное бэкендом.
 */
export function scheduleMultiplier(schedule: string, salaryNet: number): number {
  if (schedule === 'МВ') return salaryNet === 0 ? 0 : INTER_SHIFT_PAY / salaryNet;
  if (schedule === 'не принят') return 0;
  return 1;
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

/**
 * Ввод листа-покупок (4.7 приборы, 4.12 корпоративы). Расход месяца —
 * сумма «цена × количество» по строкам этого месяца. Месяц — любой месяц
 * проекта, как и у вагончиков (4.4).
 *
 * У корпоратива поле name не используется: в форме только месяц,
 * количество участников и цена за человека.
 */
export interface InputPurchases {
  items: ItemPurchase[];
}

/**
 * ГПХ сотрудников (лист 4.10). Два числа на весь проект; расход всех
 * месяцев одинаков и равен avg_count × avg_cost. Считает calcGphEmployees.
 */
export interface InputGphEmployees {
  /** Среднее количество исполнителей в месяц; допускается дробное. */
  avg_count: number;
  /** Средняя стоимость одного исполнителя за месяц. */
  avg_cost: number;
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
  /**
   * Справочные значения исполнителя, подставленные в форму (проценты).
   * Хранятся в версии бюджета как snapshot: правка справочника не меняет
   * уже сохранённые версии. Пусто — версия сохранена до появления
   * справочных значений, расчёт идёт по ставкам эталонной формы.
   */
  profit_tax_pct?: number;
  refinancing_pct?: number;
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
  /**
   * «Стоимость + ставка рефинансирования на 1–4 месяцы» (2.Бюджет!249),
   * сумма за первые четыре месяца. Показатель справочный: ни в расходы,
   * ни в прибыль, ни в налог не входит, его только показывают.
   */
  ref_rate_amount: number;
  /** Ставка, по которой посчитан ref_rate_amount, % годовых. */
  ref_rate_pct: number;
  /**
   * Операционная маржинальность за проект (2.Бюджет!G234) — наценка на
   * расходы, а не процент. При заданном ТКП форма её обнуляет.
   */
  operating_margin: number;
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
  /** Размер шрифта, тема и цвета. Пустой объект — всё по умолчанию. */
  ui_settings: UISettings;
  /** Что пользователь может хотя бы где-нибудь. Коды — store/permissions.ts. */
  permissions: string[];
}

// ─── БДР и БДДС ─────────────────────────────────────────────────────────────

/**
 * Строка отчёта. Иерархия задана кодом («2.2.1» лежит внутри «2.2»), а не
 * вложенностью объектов: у отчёта плоский список статей, как в форме.
 */
export interface ReportRow {
  code: string;
  name: string;
  /** Глубина в кодификаторе: 0 у «1», 1 у «1.1», 2 у «2.2.1». */
  level: number;
  /** Строка собирает сумму вложенных, а не имеет своего источника. */
  group: boolean;
  /**
   * Статью платформа не считает — её заполняют руками прямо в отчёте.
   * Такие ячейки открыты на ввод.
   */
  manual: boolean;
  /** Значения по месяцам проекта, индекс 0 — первый месяц. */
  monthly: number[];
  total: number;
}

export interface BudgetReport {
  kind: 'bdr' | 'bdds';
  /** Горизонт отчёта — длительность проекта, а не жёсткие 12 месяцев. */
  months: number;
  month_labels: string[];
  rows: ReportRow[];
}

// ─── Напоминания ────────────────────────────────────────────────────────────

/**
 * Напоминание — заметка со сроком: когда срок наступил, оно всплывает
 * уведомлением, а при включённой почте ещё и уходит письмом.
 */
export interface Reminder {
  id: number;
  user_id: number;
  text: string;
  /** ISO-время срока. */
  remind_at: string;
  /** Заполнено — уведомление уже показывали, повторно оно не всплывёт. */
  shown_at?: string;
  done: boolean;
  created_at: string;
}

// ─── Настройки интерфейса ───────────────────────────────────────────────────

export const FONT_SIZES = {
  small: 'small',
  normal: 'normal',
  large: 'large',
} as const;

export type FontSize = (typeof FONT_SIZES)[keyof typeof FONT_SIZES];

export const FONT_SIZE_LABELS: Record<FontSize, string> = {
  small: 'Мелкий',
  normal: 'Обычный',
  large: 'Крупный',
};

export type ThemeMode = 'light' | 'dark';

/**
 * Персональные настройки интерфейса. Хранятся в учётной записи, а не в
 * браузере, чтобы человек видел свой интерфейс на любом устройстве.
 *
 * Все поля необязательные: у давних учёток объект пустой, и каждая
 * настройка падает на своё значение по умолчанию.
 */
export interface UISettings {
  font_size?: FontSize;
  theme?: ThemeMode;
  /** Фирменный цвет: им красится всё, что раньше было синим. */
  brand_color?: string;
  /** Цвет всплывающих уведомлений. */
  notice_color?: string;
}

// ─── Рыночная стоимость аренды ─────────────────────────────────────────────
// Платформа опрашивает площадки объявлений и считает по ним оценку.
// Обязателен только город; остальные поля уточняют её.

export interface RentMarketQuery {
  city: string;
  district?: string;
  rooms?: number;
  area?: number;
  floor?: number;
  elevator?: boolean | null;
  metro_minutes?: number;
}

export interface RentMarketSource {
  source: string;
  count: number;
  median: number;
  /** Пусто, если площадка ответила. Иначе — почему не ответила. */
  error?: string;
}

export interface RentMarketExample {
  source: string;
  price_month: number;
  daily?: boolean;
  rooms?: number;
  area?: number;
  title?: string;
  url?: string;
}

export interface RentMarketEstimate {
  query: RentMarketQuery;
  /** Объявлений в расчёте. Только помесячная аренда. */
  sample: number;
  model: string;
  model_reason: string;
  /** Прогноз модели для введённых параметров, ₽/мес. 0 — модель не строилась. */
  predicted: number;
  mae: number;
  p50: number;
  p95: number;
  /** Среднее по выборке без верхних пяти процентов — цена для бюджета. */
  recommended: number;
  /** Распределение цен: столбики для графика. */
  histogram: { from: number; to: number; count: number }[];
  sources: RentMarketSource[];
  examples: RentMarketExample[];
  calculated_at: string;
  cached: boolean;
}
