package budgets

// Ограничение данных для администратора проекта.
//
// АП видит бюджет только в части прочих расходов: у него один шаг
// мастера и урезанная выгрузка (строки 178–212 листа «2.Бюджет», см.
// auth.LimitedExport и export.go). Раньше это ограничение держал только
// фронтенд: право budget.view пропускало АП и к расчёту, и к отчётам, и
// ко всем вводным — то есть ровно к тем данным (ФОТ, выручка, прибыль),
// которые из его выгрузки намеренно вырезаны. Запросом мимо интерфейса
// они доставались целиком. Теперь режет сервер.
//
// Список повторяет статьи экрана «Прочие расходы»
// (frontend/src/pages/budgets/inputs/OverheadInput.tsx): экран читает
// все вводные разом и берёт из них свои — после фильтра он получает
// только их и работает как раньше.
var limitedInputKeys = map[string]bool{
	"overtime_rf":        true,
	"overtime_kg":        true,
	"internet":           true,
	"mobile":             true,
	"lab_research":       true,
	"training":           true,
	"medical":            true,
	"uniform":            true,
	"computers":          true,
	"furniture":          true,
	"office_supplies":    true,
	"postal":             true,
	"fuel":               true,
	"transport_services": true,
	"subcontract_org":    true,
	"representative":     true,
	"bank_services":      true,
	"insurance_liab":     true,
	"utilities":          true,
	"security":           true,
	"auto_insurance":     true,
}
