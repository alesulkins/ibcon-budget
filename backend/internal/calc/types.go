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

// Страны сотрудников
const (
	CountryRF  = "россия"
	CountryKG  = "киргизия"
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
	PerDiemRF    float64    `json:"per_diem_rf"`    // суточные по РФ (D9)
	PerDiemOther float64    `json:"per_diem_other"` // суточные за рубежом (D10)
	Employees    []Employee `json:"employees"`
}

// BonusType — тип премии (лист 4.1, строки 4-5 Excel)
type BonusType struct {
	Name      string  `json:"name"`       // название: "День строителя (авг)", "НГ (дек)"
	MonthNum  int     `json:"month_num"`  // номер месяца в году (1-12)
	PctOfSalary float64 `json:"pct_of_salary"` // % от оклада "на руки" (0.5 = 50%)
}

// BonusEmployee — строка премий/компенсаций (лист 4.1)
type BonusEmployee struct {
	FullName       string    `json:"full_name"`
	Country        string    `json:"country"`
	MonthlyAmounts []float64 `json:"monthly_amounts"` // суммы "на руки" по месяцам
}

// InputBonuses — данные листа 4.1 (премии и компенсации)
// BonusTypes используется в UI для ввода, Employees — вычисленные итоги
type InputBonuses struct {
	BonusTypes []BonusType    `json:"bonus_types"` // типы премий (из верхней таблицы 4.1)
	Employees  []BonusEmployee `json:"employees"`   // вычисленные суммы по сотрудникам
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
	// Базовая стоимость услуг риелтора за одну квартиру (4.2!B13)
	RealtorBase float64 `json:"realtor_base"`
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
	OpMarginPct   float64 `json:"op_margin_pct"`  // % операционной маржинальности (F234)
	ManualRevenue float64 `json:"manual_revenue"` // ручная выручка в руб (F236; 0 = авто)

	// Стоимость договора для БГ и налогов (G251; 0 = равна расчётной выручке)
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

	// Накладные расходы (строки 178-211) — двумерный массив по статьям
	TransportRental   []float64 // 180 Аренда транспорта (4.3)
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
	GarageRent        []float64 // 210 (4.3)
	AutoInsurance     []float64 // 211

	Params *InputBudgetParams
}

// ---------------------- Результаты расчёта ----------------------

// MonthlyResult — результаты по одному месяцу
type MonthlyResult struct {
	Month int // 1-indexed

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
