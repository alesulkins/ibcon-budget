package budgets

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"

	"ibcon-budget/internal/auditlog"
	"ibcon-budget/internal/auth"
	"ibcon-budget/internal/calc"
	"ibcon-budget/internal/middleware"
	"ibcon-budget/internal/projects"
	"ibcon-budget/internal/users"
)

type Handler struct {
	svc         *Service
	projectsSvc *projects.Service
	usersSvc    *users.Service
	audit       *auditlog.Service
}

func NewHandler(svc *Service, projectsSvc *projects.Service, usersSvc *users.Service, audit *auditlog.Service) *Handler {
	return &Handler{svc: svc, projectsSvc: projectsSvc, usersSvc: usersSvc, audit: audit}
}

func (h *Handler) Register(r gin.IRouter) {
	// Маршруты на уровне проекта (параметр :id = project_id)
	proj := r.Group("/projects/:id/budgets")
	proj.GET("/versions", h.listVersions)
	proj.POST("/versions", middleware.RequireRole(auth.RoleGE, auth.RoleEP, auth.RoleIP), h.createVersion)
	proj.POST("/new-version", middleware.RequireRole(auth.RoleGE, auth.RoleEP), h.newVersion)

	// Маршруты на уровне версии бюджета (параметр :vid = version_id)
	ver := r.Group("/budget-versions/:vid")
	ver.GET("", h.getVersion)
	ver.PATCH("/status", middleware.RequireRole(auth.RoleGE, auth.RoleEP), h.changeStatus)
	ver.PUT("/cost-override", middleware.RequireRole(auth.RoleGE, auth.RoleEP), h.updateCostOverride)
	ver.PUT("/inputs/:type", middleware.RequireRole(auth.RoleGE, auth.RoleEP), h.saveInput)
	ver.GET("/inputs/:type", h.getInput)
	ver.GET("/inputs", h.getAllInputs)
	ver.GET("/calculate", h.calculate)
	ver.GET("/export", h.export)
}

func (h *Handler) projectID(c *gin.Context) int {
	id, _ := strconv.Atoi(c.Param("id"))
	return id
}

func (h *Handler) versionID(c *gin.Context) int {
	id, _ := strconv.Atoi(c.Param("vid"))
	return id
}

// canAccess проверяет доступ к проекту
func (h *Handler) canAccess(claims *auth.Claims, projectID int) bool {
	if claims.Role == auth.RoleGE {
		return true
	}
	exists, _, _ := h.usersSvc.HasProjectAccess(claims.UserID, projectID)
	return exists
}

// canEdit проверяет право на редактирование
func (h *Handler) canEdit(claims *auth.Claims, projectID int) bool {
	if claims.Role == auth.RoleGE {
		return true
	}
	_, canEdit, _ := h.usersSvc.HasProjectAccess(claims.UserID, projectID)
	return canEdit
}

// canEditVersion — можно ли править данные конкретной версии.
//
// Общее право на проект даёт canEdit. Сверх него действует правило
// владельца 2026-08-27: согласованную и архивную версию нельзя менять
// напрямую — исключение сделано для АВТОРА версии и главного экономиста.
// Остальным остаётся создать новую версию копированием.
func (h *Handler) canEditVersion(claims *auth.Claims, v *BudgetVersion) (bool, string) {
	if !h.canEdit(claims, v.ProjectID) {
		return false, "нет права на редактирование"
	}
	if !IsFrozen(v.Status) {
		return true, ""
	}
	if CanEditFrozenVersion(claims.Role, claims.UserID, v.CreatedBy) {
		return true, ""
	}
	return false, "согласованную и архивную версию может править только её автор " +
		"или главный экономист — создайте новую версию копированием"
}

// checkProjectNotFrozen проверяет, что бюджет можно создавать/редактировать
func (h *Handler) checkProjectNotFrozen(projectID int) error {
	status, err := h.projectsSvc.ProjectStatus(projectID)
	if err != nil {
		return err
	}
	if projects.IsFrozenForBudget(status) {
		return nil // вернёт ошибку снаружи
	}
	return nil
}

func (h *Handler) listVersions(c *gin.Context) {
	pid := h.projectID(c)
	claims := middleware.GetClaims(c)
	if !h.canAccess(claims, pid) {
		c.JSON(http.StatusForbidden, gin.H{"error": "нет доступа"})
		return
	}
	versions, err := h.svc.ListVersions(pid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// AP видит только ограниченные данные (без финансовых показателей)
	if claims.Role == auth.RoleAP || claims.Role == auth.RoleRP || claims.Role == auth.RoleManagement {
		c.JSON(http.StatusOK, versions)
		return
	}
	c.JSON(http.StatusOK, versions)
}

func (h *Handler) createVersion(c *gin.Context) {
	pid := h.projectID(c)
	claims := middleware.GetClaims(c)

	if !h.canAccess(claims, pid) {
		c.JSON(http.StatusForbidden, gin.H{"error": "нет доступа"})
		return
	}

	// Проверяем статус проекта
	status, _ := h.projectsSvc.ProjectStatus(pid)
	if projects.IsFrozenForBudget(status) {
		c.JSON(http.StatusForbidden, gin.H{"error": "создание бюджета для проекта в данном статусе запрещено"})
		return
	}

	var req CreateVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	v, err := h.svc.CreateVersion(pid, claims.UserID, req)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	h.audit.Log(auditlog.Entry{
		UserID: &claims.UserID, UserRole: claims.Role,
		Action: "create_budget_version", ObjectType: "budget_version", ObjectID: &v.ID,
		Comment: h.versionTitle(pid, v, req.CopyFromID),
	})
	c.JSON(http.StatusCreated, v)
}

func (h *Handler) newVersion(c *gin.Context) {
	pid := h.projectID(c)
	claims := middleware.GetClaims(c)

	if !h.canEdit(claims, pid) {
		c.JSON(http.StatusForbidden, gin.H{"error": "нет права на редактирование"})
		return
	}

	status, _ := h.projectsSvc.ProjectStatus(pid)
	if projects.IsFrozenForBudget(status) {
		c.JSON(http.StatusForbidden, gin.H{"error": "создание бюджета для проекта в данном статусе запрещено"})
		return
	}

	var req CreateVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	v, err := h.svc.NewVersion(pid, claims.UserID, req)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	h.audit.Log(auditlog.Entry{
		UserID: &claims.UserID, UserRole: claims.Role,
		Action: "create_new_budget_version", ObjectType: "budget_version", ObjectID: &v.ID,
		Comment: h.versionTitle(pid, v, req.CopyFromID),
	})
	c.JSON(http.StatusCreated, v)
}

func (h *Handler) getVersion(c *gin.Context) {
	vid := h.versionID(c)
	v, err := h.svc.GetVersion(vid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "версия не найдена"})
		return
	}
	claims := middleware.GetClaims(c)
	if !h.canAccess(claims, v.ProjectID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "нет доступа"})
		return
	}
	c.JSON(http.StatusOK, v)
}

func (h *Handler) changeStatus(c *gin.Context) {
	vid := h.versionID(c)
	v, err := h.svc.GetVersion(vid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "версия не найдена"})
		return
	}
	claims := middleware.GetClaims(c)

	if !h.canEdit(claims, v.ProjectID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "нет права на редактирование"})
		return
	}

	// Только GE может согласовывать
	var req ChangeStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Status == StatusApproved && claims.Role != auth.RoleGE {
		c.JSON(http.StatusForbidden, gin.H{"error": "только главный экономист может согласовывать бюджет"})
		return
	}

	// EP может менять только свою версию
	if claims.Role == auth.RoleEP {
		owner, _ := h.svc.VersionOwner(vid)
		if owner != claims.UserID {
			c.JSON(http.StatusForbidden, gin.H{"error": "экономист проекта может менять статус только своих версий"})
			return
		}
	}

	updated, err := h.svc.ChangeStatus(vid, claims.UserID, req)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	h.audit.Log(auditlog.Entry{
		UserID: &claims.UserID, UserRole: claims.Role,
		Action: "change_budget_status", ObjectType: "budget_version", ObjectID: &vid,
		Comment: req.Status + ": " + req.Comment,
	})
	c.JSON(http.StatusOK, updated)
}

func (h *Handler) updateCostOverride(c *gin.Context) {
	vid := h.versionID(c)
	v, err := h.svc.GetVersion(vid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "версия не найдена"})
		return
	}
	claims := middleware.GetClaims(c)
	if ok, reason := h.canEditVersion(claims, v); !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": reason})
		return
	}
	var req UpdateCostOverrideRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	v, err = h.svc.UpdateCostOverride(vid, req.CostOverride, claims.UserID)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, v)
}

func (h *Handler) saveInput(c *gin.Context) {
	vid := h.versionID(c)
	v, err := h.svc.GetVersion(vid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "версия не найдена"})
		return
	}
	claims := middleware.GetClaims(c)
	if ok, reason := h.canEditVersion(claims, v); !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": reason})
		return
	}
	inputType := c.Param("type")
	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ошибка чтения тела запроса"})
		return
	}
	if !json.Valid(body) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "тело должно быть валидным JSON"})
		return
	}
	// Часть проверок зависит от проекта: от исполнителя (достижимость целевой
	// рентабельности при его ставке налога) и от длительности (месяц покупки
	// авто должен лежать внутри проекта), поэтому берём проект версии.
	var (
		executorName string
		duration     int
	)
	if proj, perr := h.projectsSvc.Get(v.ProjectID); perr == nil {
		executorName = proj.ExecutorName
		duration = proj.DurationMonths
	}
	if err = calc.ValidateInput(inputType, body, executorName, duration); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	if err = h.svc.SaveInput(vid, claims.UserID, inputType, body); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	// В журнал изменений сохранение данных НЕ пишется (решение владельца
	// 2026-08-27). На шагах мастера нет кнопки «Сохранить»: форма уходит
	// на сервер сама — по дебаунсу в секунду после правки и раз в две
	// минуты страховочно. Каждое такое обращение давало строку лога, и
	// журнал состоял из них на 84 %, скрывая осмысленные действия.
	//
	// Кто и когда правил версию, по-прежнему видно: budget_inputs хранит
	// updated_by и updated_at по каждому ключу ввода.
	c.JSON(http.StatusOK, gin.H{"message": "данные сохранены"})
}

func (h *Handler) getInput(c *gin.Context) {
	vid := h.versionID(c)
	v, err := h.svc.GetVersion(vid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "версия не найдена"})
		return
	}
	claims := middleware.GetClaims(c)
	if !h.canAccess(claims, v.ProjectID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "нет доступа"})
		return
	}
	inputType := c.Param("type")
	data, err := h.svc.GetInput(vid, inputType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", data)
}

func (h *Handler) calculate(c *gin.Context) {
	vid := h.versionID(c)
	v, err := h.svc.GetVersion(vid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "версия не найдена"})
		return
	}
	claims := middleware.GetClaims(c)
	if !h.canAccess(claims, v.ProjectID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "нет доступа"})
		return
	}

	// Получаем проект для start_date, duration и executor
	proj, err := h.projectsSvc.Get(v.ProjectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "проект не найден"})
		return
	}

	// Загружаем все входные данные
	rawInputs, err := h.svc.GetAllInputs(vid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	inp, err := calc.LoadInputs(rawInputs, proj.StartDate, proj.DurationMonths, proj.ExecutorName)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	result := calc.Run(inp)

	// Кэшируем итоги в budget_versions
	_ = h.svc.UpdateCachedResults(vid, &result.TotalCosts, &result.Profitability)

	c.JSON(http.StatusOK, result)
}

func (h *Handler) getAllInputs(c *gin.Context) {
	vid := h.versionID(c)
	v, err := h.svc.GetVersion(vid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "версия не найдена"})
		return
	}
	claims := middleware.GetClaims(c)
	if !h.canAccess(claims, v.ProjectID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "нет доступа"})
		return
	}
	data, err := h.svc.GetAllInputs(vid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Собираем в map[string]json.RawMessage для нормального JSON-ответа
	result := make(map[string]json.RawMessage, len(data))
	for k, v := range data {
		result[k] = json.RawMessage(v)
	}
	c.JSON(http.StatusOK, result)
}

// export отдаёт версию бюджета книгой xlsx.
//
// Выгрузка — осознанное действие человека, поэтому пишется в журнал
// изменений (в отличие от автосохранения, которое из журнала убрано).
func (h *Handler) export(c *gin.Context) {
	vid := h.versionID(c)
	v, err := h.svc.GetVersion(vid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "версия не найдена"})
		return
	}
	claims := middleware.GetClaims(c)
	if !h.canAccess(claims, v.ProjectID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "нет доступа"})
		return
	}

	proj, err := h.projectsSvc.Get(v.ProjectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "проект не найден"})
		return
	}

	rawInputs, err := h.svc.GetAllInputs(vid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	inp, err := calc.LoadInputs(rawInputs, proj.StartDate, proj.DurationMonths, proj.ExecutorName)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	result := calc.Run(inp)

	meta := ExportMeta{
		ProjectID:      proj.ID,
		ProjectName:    proj.Name,
		Customer:       proj.Customer,
		ExecutorName:   proj.ExecutorName,
		VersionNo:      v.VersionNo,
		Status:         v.Status,
		StartDate:      proj.StartDate,
		DurationMonths: proj.DurationMonths,
	}
	if v.VersionLabel != nil {
		meta.VersionLabel = *v.VersionLabel
	}

	data, err := BuildExport(meta, result)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "не удалось собрать файл: " + err.Error()})
		return
	}

	h.audit.Log(auditlog.Entry{
		UserID: &claims.UserID, UserRole: claims.Role,
		Action: "export_budget", ObjectType: "budget_version", ObjectID: &vid,
		Comment: fmt.Sprintf("Бюджет %d.%d — %s", proj.ID, v.VersionNo, proj.Name),
	})

	name := ExportFileName(meta)
	// filename* с кодировкой UTF-8: имя файла русское, без этого браузер
	// сохранит его крякозябрами.
	c.Header("Content-Disposition",
		"attachment; filename=\"budget.xlsx\"; filename*=UTF-8''"+url.PathEscape(name))
	c.Data(http.StatusOK,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data)
}

// versionTitle — как назвать созданную версию в журнале.
// «Бюджет 9.2» или «Бюджет 9.2 — копия бюджета 9.1», если копировали.
func (h *Handler) versionTitle(projectID int, v *BudgetVersion, copyFrom *int) string {
	title := fmt.Sprintf("Бюджет %d.%d", projectID, v.VersionNo)
	if copyFrom == nil {
		return title
	}
	src, err := h.svc.GetVersion(*copyFrom)
	if err != nil {
		return title
	}
	return fmt.Sprintf("%s — копия бюджета %d.%d", title, projectID, src.VersionNo)
}
