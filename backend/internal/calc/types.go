package calc

import "time"

// Коды графиков работы (из справочника 4.6)
const (
	ScheduleMV         = "МВ"          // межвахтовый перерыв
	ScheduleK          = "К"           // командировка
	Schedule42         = "4/2"         // вахта 4/2
	ScheduleOF         = "ОФ"          // офис
	Schedule44         = "4/4"         // вахта 4/4
	ScheduleOTP        = "ОТП"         // отпуск
	ScheduleNotHired   = "не принят"
)

// Исполнители (из справочника executors)
const (
	ExecutorAibicon        = "Айбикон"
	ExecutorAibiconProject = "Айбикон-Проект"
	ExecutorAibiconKG      = "Айбикон Киргизия"
)

// Страны налогового обременения сотрудника (справочник 5.1!H4:H6).
// Значения хранятся в нижнем регистре — сравнение всегда через
// normalizeCountry, потому что Excel сравнивает регистронезависимо,
// а в форме страна записана с заглавной буквы («Россия»).
// См. CLAUDE.md → «Санкционированное отклонение №3», п.2 и п.3.
const (
	CountryRF = "россия"   // НДФЛ + страховые взносы 30.2%
	CountryKG = "киргизия" // страховые взносы 22.25%, НДФЛ нет
	// CountrySelfEmployed — самозанятый: платит налоги сам,
	// платформа не начисляет ни НДФЛ, ни взносы.
	CountrySelfEmployed = "самозанятый, без но"
)

// Режимы суммы "прочих расходов" (строка 218)
const (
	OtherExpModeMLN = "млн"
	OtherExpModePct = "%"
)

// Режимы ставки банковской гарантии
const (
	BGRatePerYear  = "%/год"
	BGRateTotal    = "%/весь срок"
)

// npflGrossUpDivisor — gross-up на НДФЛ 13% при выплате физическому лицу:
// 0.87 = 1 - 0.13. Начисленную сумму делят на этот коэффициент, чтобы
// получить затраты с учётом удерживаемого налога.
//
// Встречается в форме в трёх местах, везде — только для российских
// исполнителей (для «Айбикон Киргизия» деление не применяется):
//   - 4.2!C5  — аренда квартир:  (C22+C9)/0.87
//   - 4.5!C5  — аренда офиса:    C18/0.87
//   - 4.6!D9  — суточные РФ:     700+300/0.87*1.3
const npflGrossUpDivisor = 0.87

// ---------------------- Входные данные ----------------------

// Employee — один сотрудник (лист 4.6)
type Employee struct {
	// Информация о сотруднике
	Position     string  `json:"position"`      // должность (Специалист/Инженер ПТО и т.п.)
	FullName     string  `json:"full_name"`
	Country      string  `json:"country"`       // "россия" / "киргизия"
	BaseSchedule string  `json:"base_schedule"` // условия работы (вахта/офис и т.п.)
	SalaryNet    float64 `json:"salary_net"`    // план ФОТ на руки

	// Сетка графиков по месяцам (длина = DurationMonths)
	MonthlySchedule []string `json:"monthly_schedule"`

	// Командировочные дни по месяцам (РФ)
	TripDaysRF []int `json:"trip_days_rf"`
	// Командировочные дни по месяцам (другие страны)
	TripDaysOther []int `json:"trip_days_other"`
}

// InputEmployees — данные листа 4.6 (сотрудники и билеты)
// Билеты рассчитываются автоматически из MonthlySchedule: 4/2→2, К→2(командировка), смена→1
type InputEmployees struct {
	TicketPrice  float64    `json:"ticket_price"`   // стоимость одного билета (C5, default 40000)
	// PerDiemRF — суточные по РФ. В РАСЧЁТЕ НЕ УЧАСТВУЕТ: 4.6!D9 это
	// формула `=700+300/0.87*1.3`, а не поле ввода, поэтому применяется
	// константа perDiemRFRate. Поле оставлено, чтобы старые сохранённые
	// данные не ломали разбор JSON.
	PerDiemRF float64 `json:"per_diem_rf"`
	PerDiemOther float64    `json:"per_diem_other"` // суточные за рубежом (D10)
	Employees    []Employee `json:"employees"`
}

// Виды премий (лист 4.1, верхняя таблица). «Другое» — произвольная премия,
// у которой название, месяц и процент задаёт пользователь.
const (
	BonusKindBuilderDay = "день_строителя"
	BonusKindNewYear    = "новый_год"
	BonusKindOther      = "другое"
)

// Значения по умолчанию для стандартных премий.
// Это ДЕФОЛТЫ уровня проекта, а не жёсткие константы: экономист может
// выставить в проекте свои процент и месяц (CLAUDE.md, отклонение №2).
const (
	defaultBuilderDayPct   = 20.0 // % от оклада
	defaultBuilderDayMonth = 8    // август
	defaultNewYearPct      = 50.0 // % от оклада
	defaultNewYearMonth    = 12   // декабрь
)

// BonusType — один вид премии на уровне проекта (лист 4.1, строки 4-8).
type BonusType struct {
	Kind string `json:"kind"` // день_строителя / новый_год / другое
	Name string `json:"name"` // отображаемое название
	// MonthNum — КАЛЕНДАРНЫЙ месяц начисления (1-12), не месяц проекта.
	// 0 → взять значение по умолчанию для Kind.
	MonthNum int `json:"month_num"`
	// PctOfSalary — процент от оклада ЧИСЛОМ: 20 означает 20%
	// (как UnpredictablesPct/AUPPct в InputBudgetParams).
	// 0 → взять значение по умолчанию для Kind.
	PctOfSalary float64 `json:"pct_of_salary"`
}

// InputBonuses — данные листа 4.1 (премии и компенсации при увольнении).
// Суммы НЕ приходят готовыми: они считаются в calcBonuses из этих видов
// премий и списка сотрудников (санкционированное отклонение №2).
type InputBonuses struct {
	BonusTypes []BonusType `json:"bonus_types"`
}

// BonusConflict — двум и более премиям одного сотрудника выпал один месяц.
// Не ошибка: начисляются обе (суммируются), но требуется подтверждение
// пользователя (CLAUDE.md, отклонение №2). UI-подтверждение — этап B.
type BonusConflict struct {
	EmployeeName string   `json:"employee_name"`
	MonthIdx     int      `json:"month_idx"` // месяц проекта, 1-based
	BonusNames   []string `json:"bonus_names"`
}

// BonusCalcResult — премии и компенсации по месяцам (2.Бюджет строка 169).
// Разбивка по странам нужна для налогов: НДФЛ и взносы РФ считаются от
// сумм сотрудников «россия», взносы КГ — от «киргизия».
type BonusCalcResult struct {
	Total     []float64       // все сотрудники, по месяцам
	RF        []float64       // только сотрудники «россия»
	KG        []float64       // только сотрудники «киргизия»
	Conflicts []BonusConflict // где нужно подтверждение пользователя
}

// InputRentApartments — данные листа 4.2 «Аренда квартир».
//
// Экономист вводит количество квартир по типам и месяцам, цену аренды по
// каждому типу, базовую стоимость уборки и базовую стоимость услуг риелтора.
// Итоговые суммы для 2.Бюджет считает calcRentApartments:
//   - «Итого аренда» (4.2!строка 5, включает уборку) → 2.Бюджет строка 178
//   - «Риелтор»      (4.2!строка 13)                 → 2.Бюджет строка 179
//
// Уборка отдельной строкой в 2.Бюджет НЕ уходит — она входит в аренду.
type InputRentApartments struct {
	// Цена аренды одной квартиры за месяц по типам (4.2!B19, B20, B21)
	Price1Room float64 `json:"price_1room"`
	Price2Room float64 `json:"price_2room"`
	Price3Room float64 `json:"price_3room"`

	// Количество квартир по месяцам (4.2!C19:BJ19, C20:BJ20, C21:BJ21)
	Count1Room []int `json:"count_1room"`
	Count2Room []int `json:"count_2room"`
	Count3Room []int `json:"count_3room"`

	// Базовая стоимость уборки одной квартиры за месяц (4.2!B9)
	CleaningBase float64 `json:"cleaning_base"`

	// CleaningMonths — номера месяцев проекта (1-based), в которых
	// начисляется уборка. Пустой список — уборки нет за весь период.
	//
	// РАСШИРЕНИЕ СВЕРХ ФОРМЫ: в Excel уборка начисляется безусловно каждый
	// месяц (4.2!C9 = IF(месяц<=D8, $B$9*(C19+C20+C21), 0)), выбора месяцев
	// там нет. Поведение затребовано владельцем 2026-08-23.
	CleaningMonths []int `json:"cleaning_months"`
	// Базовая стоимость услуг риелтора за одну квартиру (4.2!B13)
	RealtorBase float64 `json:"realtor_base"`
}

// CarPurchase — одна покупка авто, строка таблицы 4.3!A17:D27.
//
// Итог строки, как и в форме: `количество × стоимость` (4.3!D17 =
// IF(A17<=D8, C17*B17, 0)).
type CarPurchase struct {
	// Описание авто. В форме такого поля нет — добавлено, чтобы строку
	// таблицы можно было опознать.
	Name string `json:"name"`
	// Месяц покупки, 1-based (4.3!A17:A27). 0 — строка не заполнена и
	// целиком игнорируется; месяц вне проекта отклоняет ValidateTransport.
	Month int `json:"month"`
	// Количество покупаемых авто (4.3!B17:B27)
	Count int `json:"count"`
	// Стоимость одного авто (4.3!C17:C27)
	Price float64 `json:"price"`
}

// RentedItem — одна строка аренды: авто (4.3 строки 5-6) или гараж
// (4.3 строки 10-11).
//
// Отличие от формы: там цена одна на весь вид (4.3!B6 для авто, B11 для
// гаража), а количество задаётся отдельно в каждом месяце (4.3!C5:BJ5,
// C10:BJ10). Здесь строка несёт свою цену, своё количество и набор
// месяцев, в которых это количество арендуется, — так у разных машин могут
// быть разные цены. Помесячно меняющееся количество выражается несколькими
// строками; итоговые суммы совпадают с формой.
type RentedItem struct {
	Name string `json:"name"`
	// Цена аренды одной единицы за месяц (4.3!B6 / 4.3!B11)
	Price float64 `json:"price"`
	// Количество единиц (4.3!C5:BJ5 / 4.3!C10:BJ10).
	// 0 — строка ничего не начисляет, это допустимое значение.
	Count int `json:"count"`
	// Номера месяцев проекта (1-based), в которых предмет арендуется
	Months []int `json:"months"`
}

// InputTransport — данные листа 4.3 «Транспорт».
//
// Покупка авто и аренда авто уходят ОДНОЙ строкой 180 «Аренда транспорта»
// (так устроена форма), аренда гаража — отдельной строкой 210.
// Считает calcTransport.
type InputTransport struct {
	CarPurchases  []CarPurchase `json:"car_purchases"`
	CarRentals    []RentedItem  `json:"car_rentals"`
	GarageRentals []RentedItem  `json:"garage_rentals"`
}

// InputMonthlyCosts — простые ежемесячные затраты (используется для большинства статей 4.2-4.12)
// Ключ = название строки, значение = массив сумм по месяцам
type InputMonthlyCosts struct {
	Lines []CostLine `json:"lines"`
}

type CostLine struct {
	Name           string    `json:"name"`
	MonthlyAmounts []float64 `json:"monthly_amounts"`
}

// BankGuarantee — параметры одной банковской гарантии
type BankGuarantee struct {
	Pct          float64 `json:"pct"`           // % от стоимости договора (F220/F224/F228)
	RatePct      float64 `json:"rate_pct"`      // % ставка (F221/F225/F229)
	RateType     string  `json:"rate_type"`     // "%/год" или "%/весь срок"
	DurationMos  float64 `json:"duration_mos"`  // срок в месяцах (F222/F226/F230)
}

// InputBudgetParams — параметры бюджета (шаги 3 и 22 визарда)
type InputBudgetParams struct {
	// Непредвиденные, АУП (строки 214-215)
	UnpredictablesPct float64 `json:"unpredictables_pct"` // % от ФОТ+расходов (F214)
	AUPPct            float64 `json:"aup_pct"`            // % АУП (F215)

	// Прочие расходы (строка 218)
	OtherExpenseMode  string  `json:"other_expense_mode"`  // "млн" или "%"
	OtherExpenseValue float64 `json:"other_expense_value"` // значение

	// Банковские гарантии
	BGExecution BankGuarantee `json:"bg_execution"` // БГ на исполнение обязательств
	BGWarranty  BankGuarantee `json:"bg_warranty"`  // БГ на гарантийный период
	BGAdvance   BankGuarantee `json:"bg_advance"`   // БГ на аванс

	// Выручка
	// TargetRentPct — целевая рентабельность БЕЗ налога на прибыль, в процентах
	// (20 означает 20%). Источник: 2.Бюджет!E234, ручной ввод экономиста.
	//
	// Сам коэффициент наценки на расходы больше НЕ вводится: он считается из
	// целевой рентабельности и ставки налога исполнителя (2.Бюджет!F234),
	// см. markupRate. Работает только в режиме наценки — когда ТКП
	// (ContractValue) не задан.
	TargetRentPct float64 `json:"target_rent_pct"`

	// ContractValue — ТКП, стоимость договора (2.Бюджет!G251). ЕДИНСТВЕННЫЙ
	// ручной ввод в этом блоке формы.
	//
	// Отдельного поля «ручная стоимость работ» в форме НЕТ: F236 это
	// формула `=G252`, а G252 = `IF(КГ, G251, IF(АП, G251, G251/1.22))`,
	// то есть ТКП, приведённый к сумме без НДС. Раньше платформа держала
	// ManualRevenue отдельным вводом — экономист заполнял одно и то же
	// значение дважды, а деление на 1.22 для «Айбикон» не выполнялось
	// вовсе, из-за чего выручка завышалась на 22%. См. contractNetOfVAT.
	ContractValue float64 `json:"contract_value"`
}

// OverrideMonthly — явное задание статьи накладных по месяцам (строки 186-211 без расчётных)
type OverrideMonthly struct {
	MonthlyAmounts []float64 `json:"monthly_amounts"`
}

// ---------------------- Входные данные для одного расчёта ----------------------

// BudgetInputs — все входные данные одной версии бюджета
type BudgetInputs struct {
	ProjectStartDate time.Time `json:"-"` // из projects.start_date
	DurationMonths   int       `json:"-"` // из projects.duration_months
	ExecutorName     string    `json:"-"` // из projects.executor

	Employees       *InputEmployees   // 4.6
	Bonuses         *InputBonuses     // 4.1 (премии)
	OvertimeRF      []float64         // строка 170: переработки сотрудников РФ
	OvertimeKG      []float64         // строка 171: переработки сотрудников Киргизии

	// Лист 4.2 — считается по формуле, а не приходит готовой суммой.
	// Даёт строки 178 (аренда, вкл. уборку) и 179 (риелтор).
	RentApts *InputRentApartments

	// Лист 4.3 — считается по формуле. Даёт строки 180 (аренда транспорта,
	// включая покупку авто) и 210 (аренда гаража).
	Transport *InputTransport

	// Накладные расходы (строки 178-211) — двумерный массив по статьям
	//
	// TransportRental и GarageRent — СТАРЫЙ формат листа 4.3 (готовые суммы
	// по месяцам). Используются только как запасной путь для версий бюджета,
	// сохранённых до перехода на расчёт по формуле: если у версии есть
	// Transport, эти поля игнорируются. Удалить, когда старых версий не
	// останется.
	TransportRental   []float64 // 180 Аренда транспорта (4.3), устаревший ввод
	SiteSetup         []float64 // 181 Обустройство стройплощадки (4.4)
	OfficeRent        []float64 // 182 Аренда офиса (4.5)
	OfficeCleaning    []float64 // 183 Уборка офиса (4.5)
	// 184 Билеты — рассчитывается из Employee.MonthlySchedule
	// 185 Командировочные — рассчитывается из Employee.TripDays
	Internet          []float64 // 186
	Mobile            []float64 // 187
	LabResearch       []float64 // 188
	ControlEquipment  []float64 // 189 (4.7)
	Training          []float64 // 190
	Medical           []float64 // 191
	Uniform           []float64 // 192
	Software          []float64 // 193 (4.8)
	Computers         []float64 // 194 Приобретение ПК + оргтехника
	Furniture         []float64 // 195
	OfficeSupplies    []float64 // 196
	Postal            []float64 // 197
	Fuel              []float64 // 198 ГСМ
	TransportServices []float64 // 199
	SubcontractExt    []float64 // 200 (4.9)
	SubcontractEmp    []float64 // 201 (4.10)
	SubcontractGen    []float64 // 202 (4.11)
	SubcontractOrg    []float64 // 203
	Representative    []float64 // 204
	CorporateEvents   []float64 // 205 (4.12)
	BankServices      []float64 // 206
	InsuranceLiab     []float64 // 207
	Utilities         []float64 // 208
	Security          []float64 // 209
	GarageRent        []float64 // 210 (4.3), устаревший ввод — см. выше
	AutoInsurance     []float64 // 211

	Params *InputBudgetParams
}

// ---------------------- Результаты расчёта ----------------------

// MonthlyResult — результаты по одному месяцу
type MonthlyResult struct {
	// Без json-тега поле уезжало в JSON как "Month", и фронтенд, читавший
	// month, получал undefined — помесячная разбивка показывала
	// «Invalid Date» вместо названий месяцев.
	Month int `json:"month"` // 1-indexed

	// ФОТ и налоги (строки 168-176)
	FOT            float64 `json:"fot"`              // 168
	Bonuses        float64 `json:"bonuses"`           // 169
	OvertimeRF     float64 `json:"overtime_rf"`       // 170
	OvertimeKG     float64 `json:"overtime_kg"`       // 171
	NDFL           float64 `json:"ndfl"`              // 172
	InsuranceRF    float64 `json:"insurance_rf"`      // 173
	InsuranceKG    float64 `json:"insurance_kg"`      // 174
	TotalFOT       float64 `json:"total_fot"`         // 176 = SUM(168-174)

	// Накладные (178-211)
	Overhead [34]float64 `json:"overhead"` // [0]=178, [1]=179, ..., [33]=211
	Tickets  float64     `json:"tickets"`  // 184
	PerDiem  float64     `json:"per_diem"` // 185

	// Итоги
	ProjectCostsExFOT float64 `json:"project_costs_ex_fot"` // 212 = SUM(178-211)
	Unpredictables    float64 `json:"unpredictables"`        // 214
	AUP               float64 `json:"aup"`                   // 215
	TotalCostsGross   float64 `json:"total_costs_gross"`     // 216 = 215+214+212+176
	OtherExpenses     float64 `json:"other_expenses"`        // 218
	BGExecution       float64 `json:"bg_execution"`          // 222
	BGWarranty        float64 `json:"bg_warranty"`           // 226
	BGAdvance         float64 `json:"bg_advance"`            // 230
	TotalCosts        float64 `json:"total_costs"`           // 232 = 230+226+218+216+222
	MarginAmount      float64 `json:"margin_amount"`         // 234
	Revenue           float64 `json:"revenue"`               // 236 (без НДС)
	OperatingProfit   float64 `json:"operating_profit"`      // 238
	RevenueWithVAT    float64 `json:"revenue_with_vat"`      // 247
}

// CalcResult — полные результаты расчёта бюджета
type CalcResult struct {
	DurationMonths int             `json:"duration_months"`
	Monthly        []MonthlyResult `json:"monthly"`

	// Итого по проекту (строки G)
	TotalFOT           float64 `json:"total_fot"`
	TotalCosts         float64 `json:"total_costs"`
	TotalRevenue       float64 `json:"total_revenue"` // G236
	OperatingProfit    float64 `json:"operating_profit"`
	Tax                float64 `json:"tax"`
	NetProfit          float64 `json:"net_profit"`
	Profitability      float64 `json:"profitability"` // % G244
	TotalRevenueWithVAT float64 `json:"total_revenue_with_vat"` // G247
}
