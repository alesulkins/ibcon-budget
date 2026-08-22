package budgets

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"ibcon-budget/internal/auth"
	"ibcon-budget/internal/auditlog"
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
	if !h.canEdit(claims, v.ProjectID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "нет права на редактирование"})
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
	if !h.canEdit(claims, v.ProjectID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "нет права на редактирование"})
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
	if err = calc.ValidateInput(inputType, body); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	if err = h.svc.SaveInput(vid, claims.UserID, inputType, body); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	h.audit.Log(auditlog.Entry{
		UserID: &claims.UserID, UserRole: claims.Role,
		Action: "save_budget_input", ObjectType: "budget_version", ObjectID: &vid,
		Comment: inputType,
	})
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
