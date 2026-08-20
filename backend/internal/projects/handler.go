package projects

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"ibcon-budget/internal/auth"
	"ibcon-budget/internal/auditlog"
	"ibcon-budget/internal/middleware"
	"ibcon-budget/internal/users"
)

type Handler struct {
	svc      *Service
	usersSvc *users.Service
	audit    *auditlog.Service
}

func NewHandler(svc *Service, usersSvc *users.Service, audit *auditlog.Service) *Handler {
	return &Handler{svc: svc, usersSvc: usersSvc, audit: audit}
}

func (h *Handler) Register(r gin.IRouter) {
	g := r.Group("/projects")
	g.GET("", h.list)
	g.POST("", middleware.RequireRole(auth.RoleGE, auth.RoleIP), h.create)
	g.GET("/:id", h.get)
	g.PUT("/:id", middleware.RequireRole(auth.RoleGE, auth.RoleEP, auth.RoleIP), h.update)
	g.PATCH("/:id/status", middleware.RequireRole(auth.RoleGE, auth.RoleEP, auth.RoleIP), h.changeStatus)
}

// canAccessProject проверяет, имеет ли пользователь доступ к проекту.
// GE — всегда. Остальные — только если проект назначен им или есть индивидуальный доступ.
func (h *Handler) canAccessProject(claims *auth.Claims, projectID int) bool {
	if claims.Role == auth.RoleGE {
		return true
	}
	exists, _, _ := h.usersSvc.HasProjectAccess(claims.UserID, projectID)
	return exists
}

func (h *Handler) list(c *gin.Context) {
	claims := middleware.GetClaims(c)
	params := ListParams{
		Search: c.Query("search"),
		Status: c.Query("status"),
	}
	params.Limit, _ = strconv.Atoi(c.DefaultQuery("limit", "50"))
	params.Offset, _ = strconv.Atoi(c.DefaultQuery("offset", "0"))

	// Не-GE видят только свои проекты
	if claims.Role != auth.RoleGE {
		ids, err := h.usersSvc.ProjectsForUser(claims.UserID)
		if err != nil || len(ids) == 0 {
			c.JSON(http.StatusOK, gin.H{"total": 0, "items": []any{}})
			return
		}
		params.UserIDs = ids
	}

	items, total, err := h.svc.List(params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"total": total, "items": items})
}

func (h *Handler) create(c *gin.Context) {
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	claims := middleware.GetClaims(c)

	// IP может создавать только проект со статусом prospect или active
	if claims.Role == auth.RoleIP {
		if req.Status != StatusProspect && req.Status != StatusActive {
			c.JSON(http.StatusForbidden, gin.H{"error": "инициатор проекта может создавать только перспективные или действующие проекты"})
			return
		}
	}

	p, err := h.svc.Create(req, claims.UserID)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	// Выдаём IP доступ к своему проекту
	if claims.Role == auth.RoleIP {
		_ = h.usersSvc.GrantProjectAccess(claims.UserID, p.ID, claims.UserID, true)
	}

	h.audit.Log(auditlog.Entry{
		UserID: &claims.UserID, UserRole: claims.Role,
		Action: "create_project", ObjectType: "project", ObjectID: &p.ID,
		Comment: p.Name,
	})
	c.JSON(http.StatusCreated, p)
}

func (h *Handler) get(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	claims := middleware.GetClaims(c)

	if !h.canAccessProject(claims, id) {
		c.JSON(http.StatusForbidden, gin.H{"error": "нет доступа к проекту"})
		return
	}

	p, err := h.svc.Get(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "проект не найден"})
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *Handler) update(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	claims := middleware.GetClaims(c)

	if !h.canAccessProject(claims, id) {
		c.JSON(http.StatusForbidden, gin.H{"error": "нет доступа к проекту"})
		return
	}

	// IP может редактировать только свои проекты
	if claims.Role == auth.RoleIP {
		owner, _ := h.svc.CreatedByUser(id)
		if owner != claims.UserID {
			c.JSON(http.StatusForbidden, gin.H{"error": "инициатор проекта может редактировать только созданные им проекты"})
			return
		}
	}

	var req UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	p, err := h.svc.Update(id, req, claims.UserID)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	h.audit.Log(auditlog.Entry{
		UserID: &claims.UserID, UserRole: claims.Role,
		Action: "update_project", ObjectType: "project", ObjectID: &id,
	})
	c.JSON(http.StatusOK, p)
}

func (h *Handler) changeStatus(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	claims := middleware.GetClaims(c)

	if !h.canAccessProject(claims, id) {
		c.JSON(http.StatusForbidden, gin.H{"error": "нет доступа к проекту"})
		return
	}

	var req ChangeStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	p, err := h.svc.ChangeStatus(id, req, claims.UserID)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	h.audit.Log(auditlog.Entry{
		UserID: &claims.UserID, UserRole: claims.Role,
		Action: "change_project_status", ObjectType: "project", ObjectID: &id,
		Comment: req.Status + ": " + req.Comment,
	})
	c.JSON(http.StatusOK, p)
}
