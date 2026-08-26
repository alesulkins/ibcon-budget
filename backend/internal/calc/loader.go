package calc

import (
	"encoding/json"
	"fmt"
	"time"
)

// inputTypes — список всех типов входных данных (ключи в budget_inputs)
const (
	TypeEmployees        = "employees"
	TypeBonuses          = "bonuses"
	TypeOvertimeRF       = "overtime_rf"
	TypeOvertimeKG       = "overtime_kg"
	// TypeRentApartments — лист 4.2. Структура НЕ {monthly_amounts},
	// а количество квартир по типам + цены (см. InputRentApartments).
	// Строка «Риелтор» (179) считается отсюда же, отдельного типа ввода нет.
	TypeRentApartments = "rent_apartments"
	// TypeTransport — лист 4.3. Структура НЕ {monthly_amounts}: покупки авто,
	// аренда авто и аренда гаража отдельными таблицами (см. InputTransport).
	// Даёт строки 180 (аренда транспорта + покупка) и 210 (гараж).
	TypeTransport = "transport"
	// TypeTransportRental / TypeGarageRent — СТАРЫЙ ввод листа 4.3 готовыми
	// суммами по месяцам. Читаются только ради версий, сохранённых до
	// перехода на расчёт по формуле; новые данные пишутся в TypeTransport.
	TypeTransportRental  = "transport_rental"
	TypeSiteSetup        = "site_setup"
	TypeOfficeRent       = "office_rent"
	TypeOfficeCleaning   = "office_cleaning"
	TypeInternet         = "internet"
	TypeMobile           = "mobile"
	TypeLabResearch      = "lab_research"
	TypeControlEquipment = "control_equipment"
	TypeTraining         = "training"
	TypeMedical          = "medical"
	TypeUniform          = "uniform"
	TypeSoftware         = "software"
	TypeComputers        = "computers"
	TypeFurniture        = "furniture"
	TypeOfficeSupplies   = "office_supplies"
	TypePostal           = "postal"
	TypeFuel             = "fuel"
	TypeTransportSvc     = "transport_services"
	TypeSubcontractExt   = "subcontract_ext"
	TypeSubcontractEmp   = "subcontract_emp"
	TypeSubcontractGen   = "subcontract_gen"
	TypeSubcontractOrg   = "subcontract_org"
	TypeRepresentative   = "representative"
	TypeCorporateEvents  = "corporate_events"
	TypeBankServices     = "bank_services"
	TypeInsuranceLiab    = "insurance_liab"
	TypeUtilities        = "utilities"
	TypeSecurity         = "security"
	TypeGarageRent       = "garage_rent"
	TypeAutoInsurance    = "auto_insurance"
	TypeBudgetParams     = "budget_params"
)

// simpleMonthlyCostTypes — типы с простой структурой {monthly_amounts:[...]}
var simpleMonthlyCostTypes = map[string]func(*BudgetInputs) *[]float64{
	TypeOvertimeRF:       func(b *BudgetInputs) *[]float64 { return &b.OvertimeRF },
	TypeOvertimeKG:       func(b *BudgetInputs) *[]float64 { return &b.OvertimeKG },
	TypeTransportRental:  func(b *BudgetInputs) *[]float64 { return &b.TransportRental },
	TypeSiteSetup:        func(b *BudgetInputs) *[]float64 { return &b.SiteSetup },
	TypeOfficeRent:       func(b *BudgetInputs) *[]float64 { return &b.OfficeRent },
	TypeOfficeCleaning:   func(b *BudgetInputs) *[]float64 { return &b.OfficeCleaning },
	TypeInternet:         func(b *BudgetInputs) *[]float64 { return &b.Internet },
	TypeMobile:           func(b *BudgetInputs) *[]float64 { return &b.Mobile },
	TypeLabResearch:      func(b *BudgetInputs) *[]float64 { return &b.LabResearch },
	TypeControlEquipment: func(b *BudgetInputs) *[]float64 { return &b.ControlEquipment },
	TypeTraining:         func(b *BudgetInputs) *[]float64 { return &b.Training },
	TypeMedical:          func(b *BudgetInputs) *[]float64 { return &b.Medical },
	TypeUniform:          func(b *BudgetInputs) *[]float64 { return &b.Uniform },
	TypeSoftware:         func(b *BudgetInputs) *[]float64 { return &b.Software },
	TypeComputers:        func(b *BudgetInputs) *[]float64 { return &b.Computers },
	TypeFurniture:        func(b *BudgetInputs) *[]float64 { return &b.Furniture },
	TypeOfficeSupplies:   func(b *BudgetInputs) *[]float64 { return &b.OfficeSupplies },
	TypePostal:           func(b *BudgetInputs) *[]float64 { return &b.Postal },
	TypeFuel:             func(b *BudgetInputs) *[]float64 { return &b.Fuel },
	TypeTransportSvc:     func(b *BudgetInputs) *[]float64 { return &b.TransportServices },
	TypeSubcontractExt:   func(b *BudgetInputs) *[]float64 { return &b.SubcontractExt },
	TypeSubcontractEmp:   func(b *BudgetInputs) *[]float64 { return &b.SubcontractEmp },
	TypeSubcontractGen:   func(b *BudgetInputs) *[]float64 { return &b.SubcontractGen },
	TypeSubcontractOrg:   func(b *BudgetInputs) *[]float64 { return &b.SubcontractOrg },
	TypeRepresentative:   func(b *BudgetInputs) *[]float64 { return &b.Representative },
	TypeCorporateEvents:  func(b *BudgetInputs) *[]float64 { return &b.CorporateEvents },
	TypeBankServices:     func(b *BudgetInputs) *[]float64 { return &b.BankServices },
	TypeInsuranceLiab:    func(b *BudgetInputs) *[]float64 { return &b.InsuranceLiab },
	TypeUtilities:        func(b *BudgetInputs) *[]float64 { return &b.Utilities },
	TypeSecurity:         func(b *BudgetInputs) *[]float64 { return &b.Security },
	TypeGarageRent:       func(b *BudgetInputs) *[]float64 { return &b.GarageRent },
	TypeAutoInsurance:    func(b *BudgetInputs) *[]float64 { return &b.AutoInsurance },
}

// simpleCostJSON — минимальная структура для десериализации простых статей
type simpleCostJSON struct {
	MonthlyAmounts []float64 `json:"monthly_amounts"`
}

// LoadInputs десериализует raw-JSONB данные из БД в BudgetInputs.
// projectStartDate, durationMonths, executorName — берутся из таблицы projects.
func LoadInputs(
	rawInputs map[string][]byte,
	projectStartDate time.Time,
	durationMonths int,
	executorName string,
) (*BudgetInputs, error) {
	inp := &BudgetInputs{
		ProjectStartDate: projectStartDate,
		DurationMonths:   durationMonths,
		ExecutorName:     executorName,
	}

	for typ, raw := range rawInputs {
		if len(raw) == 0 || string(raw) == "{}" || string(raw) == "null" {
			continue
		}

		switch typ {
		case TypeEmployees:
			var v InputEmployees
			if err := json.Unmarshal(raw, &v); err != nil {
				return nil, fmt.Errorf("parse %s: %w", typ, err)
			}
			inp.Employees = &v

		case TypeBonuses:
			var v InputBonuses
			if err := json.Unmarshal(raw, &v); err != nil {
				return nil, fmt.Errorf("parse %s: %w", typ, err)
			}
			inp.Bonuses = &v

		case TypeRentApartments:
			var v InputRentApartments
			if err := json.Unmarshal(raw, &v); err != nil {
				return nil, fmt.Errorf("parse %s: %w", typ, err)
			}
			inp.RentApts = &v

		case TypeTransport:
			var v InputTransport
			if err := json.Unmarshal(raw, &v); err != nil {
				return nil, fmt.Errorf("parse %s: %w", typ, err)
			}
			inp.Transport = &v

		case TypeBudgetParams:
			var v InputBudgetParams
			if err := json.Unmarshal(raw, &v); err != nil {
				return nil, fmt.Errorf("parse %s: %w", typ, err)
			}
			inp.Params = &v

		default:
			if fieldPtr, ok := simpleMonthlyCostTypes[typ]; ok {
				var v simpleCostJSON
				if err := json.Unmarshal(raw, &v); err != nil {
					return nil, fmt.Errorf("parse %s: %w", typ, err)
				}
				*fieldPtr(inp) = v.MonthlyAmounts
			}
		}
	}

	return inp, nil
}
