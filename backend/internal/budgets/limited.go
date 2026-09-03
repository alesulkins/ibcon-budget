package budgets

// Ограничение данных для администратора проекта.
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
