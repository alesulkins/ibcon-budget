package references

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"ibcon-budget/internal/access"
	"ibcon-budget/internal/auditlog"
	"ibcon-budget/internal/auth"
	"ibcon-budget/internal/middleware"
)

type Handler struct {
	svc   *Service
	acl   *access.Service
	audit *auditlog.Service
}

func NewHandler(svc *Service, acl *access.Service, audit *auditlog.Service) *Handler {
	return &Handler{svc: svc, acl: acl, audit: audit}
}

func (h *Handler) Register(r gin.IRouter) {
	refs := r.Group("/references")
	// Просмотр — всем авторизованным
	refs.GET("/executors", h.listExecutors)
	refs.GET("/positions", h.listPositions)
	refs.GET("/work-modes", h.listWorkModes)
	refs.GET("/cost-items", h.listCostItems)
	// Редактирование — по праву, а не по роли: главный экономист имеет
	// его от роли, остальным его может выдать он сам.
	ge := refs.Group("", h.acl.Require(auth.PermReferencesEdit))
	ge.POST("/executors", h.createExecutor)
	ge.PUT("/executors/:id", h.updateExecutor)
	ge.POST("/positions", h.createPosition)
	ge.PUT("/positions/:id", h.updatePosition)
	ge.POST("/work-modes", h.createWorkMode)
	ge.PUT("/work-modes/:id", h.updateWorkMode)
	ge.POST("/cost-items", h.createCostItem)
	ge.PUT("/cost-items/:id", h.updateCostItem)

	// Города и оклады должности по городам. Список городов читают формы
	// должности, поэтому просмотр — всем авторизованным.
	refs.GET("/cities", h.listCities)
	ge.POST("/cities", h.createCity)
	ge.PUT("/positions/:id/salaries", h.setCitySalaries)
}

// activeOnly — по умолчанию отдаём только активные записи: списки
// справочников читают формы мастера, а там неактивная запись выбираться
// не должна. Экран управления справочниками просит всё явно (?all=true).
func activeOnly(c *gin.Context) bool {
	return c.Query("all") != "true" && c.Query("active") != "false"
}

// ---- Executors ----

func (h *Handler) listExecutors(c *gin.Context) {
	rows, err := h.svc.ListExecutors(activeOnly(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (h *Handler) createExecutor(c *gin.Context) {
	var body struct {
		Name            string  `json:"name"      binding:"required"`
		FullName        string  `json:"full_name" binding:"required"`
		ProfitTaxRate   float64 `json:"profit_tax_rate"`
		RefinancingRate float64 `json:"refinancing_rate"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cl := middleware.GetClaims(c)
	e, err := h.svc.CreateExecutor(body.Name, body.FullName, body.ProfitTaxRate, body.RefinancingRate, cl.UserID)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	h.audit.Log(auditlog.Entry{UserID: &cl.UserID, UserRole: cl.Role, Action: "create_executor", ObjectType: "executor", ObjectID: &e.ID})
	c.JSON(http.StatusCreated, e)
}

func (h *Handler) updateExecutor(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		Name            *string  `json:"name"`
		FullName        *string  `json:"full_name"`
		ProfitTaxRate   *float64 `json:"profit_tax_rate"`
		RefinancingRate *float64 `json:"refinancing_rate"`
		Active          *bool    `json:"active"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cl := middleware.GetClaims(c)
	e, err := h.svc.UpdateExecutor(id, body.Name, body.FullName, body.ProfitTaxRate, body.RefinancingRate, body.Active, cl.UserID)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	h.audit.Log(auditlog.Entry{UserID: &cl.UserID, UserRole: cl.Role, Action: "update_executor", ObjectType: "executor", ObjectID: &id})
	c.JSON(http.StatusOK, e)
}

// ---- Positions ----

func (h *Handler) listPositions(c *gin.Context) {
	rows, err := h.svc.ListPositions(activeOnly(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (h *Handler) createPosition(c *gin.Context) {
	var body struct {
		Name   string  `json:"name" binding:"required"`
		Salary float64 `json:"salary"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cl := middleware.GetClaims(c)
	p, err := h.svc.CreatePosition(body.Name, body.Salary, cl.UserID)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	h.audit.Log(auditlog.Entry{UserID: &cl.UserID, UserRole: cl.Role, Action: "create_position", ObjectType: "position", ObjectID: &p.ID})
	c.JSON(http.StatusCreated, p)
}

func (h *Handler) updatePosition(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		Name   *string  `json:"name"`
		Salary *float64 `json:"salary"`
		Active *bool    `json:"active"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cl := middleware.GetClaims(c)
	p, err := h.svc.UpdatePosition(id, body.Name, body.Salary, body.Active, cl.UserID)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	h.audit.Log(auditlog.Entry{UserID: &cl.UserID, UserRole: cl.Role, Action: "update_position", ObjectType: "position", ObjectID: &id})
	c.JSON(http.StatusOK, p)
}

// ---- WorkModes ----

func (h *Handler) listWorkModes(c *gin.Context) {
	rows, err := h.svc.ListWorkModes(activeOnly(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (h *Handler) createWorkMode(c *gin.Context) {
	var body struct {
		Code     string `json:"code"      binding:"required"`
		FullName string `json:"full_name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cl := middleware.GetClaims(c)
	wm, err := h.svc.CreateWorkMode(body.Code, body.FullName, cl.UserID)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	h.audit.Log(auditlog.Entry{UserID: &cl.UserID, UserRole: cl.Role, Action: "create_work_mode", ObjectType: "work_mode", ObjectID: &wm.ID})
	c.JSON(http.StatusCreated, wm)
}

func (h *Handler) updateWorkMode(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		Code     *string `json:"code"`
		FullName *string `json:"full_name"`
		Active   *bool   `json:"active"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cl := middleware.GetClaims(c)
	wm, err := h.svc.UpdateWorkMode(id, body.Code, body.FullName, body.Active, cl.UserID)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	h.audit.Log(auditlog.Entry{UserID: &cl.UserID, UserRole: cl.Role, Action: "update_work_mode", ObjectType: "work_mode", ObjectID: &id})
	c.JSON(http.StatusOK, wm)
}

// ---- CostItems ----

func (h *Handler) listCostItems(c *gin.Context) {
	rows, err := h.svc.ListCostItems(activeOnly(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (h *Handler) createCostItem(c *gin.Context) {
	var body struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cl := middleware.GetClaims(c)
	ci, err := h.svc.CreateCostItem(body.Name, cl.UserID)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	h.audit.Log(auditlog.Entry{UserID: &cl.UserID, UserRole: cl.Role, Action: "create_cost_item", ObjectType: "cost_item", ObjectID: &ci.ID})
	c.JSON(http.StatusCreated, ci)
}

func (h *Handler) updateCostItem(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		Name   *string `json:"name"`
		Active *bool   `json:"active"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cl := middleware.GetClaims(c)
	ci, err := h.svc.UpdateCostItem(id, body.Name, body.Active, cl.UserID)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	h.audit.Log(auditlog.Entry{UserID: &cl.UserID, UserRole: cl.Role, Action: "update_cost_item", ObjectType: "cost_item", ObjectID: &id})
	c.JSON(http.StatusOK, ci)
}

// ---- Cities ----

func (h *Handler) listCities(c *gin.Context) {
	rows, err := h.svc.ListCities(activeOnly(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (h *Handler) createCity(c *gin.Context) {
	var body struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cl := middleware.GetClaims(c)
	city, err := h.svc.CreateCity(strings.TrimSpace(body.Name), cl.UserID)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	h.audit.Log(auditlog.Entry{
		UserID: &cl.UserID, UserRole: cl.Role,
		Action: "create_city", ObjectType: "city", ObjectID: &city.ID,
		Comment: city.Name,
	})
	c.JSON(http.StatusCreated, city)
}

// setCitySalaries заменяет оклады должности по городам целиком: город,
// пропавший из списка, теряет свою ставку — иначе снятую строку было бы
// нечем удалить.
func (h *Handler) setCitySalaries(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		Salaries []CitySalary `json:"salaries"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	for _, cs := range body.Salaries {
		if cs.Salary < 0 {
			c.JSON(http.StatusUnprocessableEntity,
				gin.H{"error": "оклад не может быть отрицательным"})
			return
		}
	}

	cl := middleware.GetClaims(c)
	if err := h.svc.SetCitySalaries(id, body.Salaries, cl.UserID); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	h.audit.Log(auditlog.Entry{
		UserID: &cl.UserID, UserRole: cl.Role,
		Action: "update_position", ObjectType: "position", ObjectID: &id,
		Comment: "оклады по городам",
	})

	rows, err := h.svc.ListPositions(false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for _, p := range rows {
		if p.ID == id {
			c.JSON(http.StatusOK, p)
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "должность не найдена"})
}
