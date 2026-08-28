package projects

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"ibcon-budget/internal/access"
	"ibcon-budget/internal/auditlog"
	"ibcon-budget/internal/auth"
	"ibcon-budget/internal/middleware"
	"ibcon-budget/internal/users"
)

type Handler struct {
	svc      *Service
	usersSvc *users.Service
	acl      *access.Service
	audit    *auditlog.Service
}

func NewHandler(svc *Service, usersSvc *users.Service, acl *access.Service, audit *auditlog.Service) *Handler {
	return &Handler{svc: svc, usersSvc: usersSvc, acl: acl, audit: audit}
}

func (h *Handler) Register(r gin.IRouter) {
	g := r.Group("/projects")
	g.GET("", h.list)
	// Создание проекта не привязано к проекту, поэтому проверяется
	// посредником. Остальные маршруты проверяют право внутри: им нужен
	// id проекта, без него ответ был бы «можно вообще», а не «можно тут».
	g.POST("", h.acl.Require(auth.PermProjectCreate), h.create)
	g.GET("/:id", h.get)
	g.PUT("/:id", h.update)
	g.PATCH("/:id/status", h.changeStatus)
}

func (h *Handler) list(c *gin.Context) {
	claims := middleware.GetClaims(c)
	params := ListParams{
		Search: c.Query("search"),
		Status: c.Query("status"),
	}
	params.Limit, _ = strconv.Atoi(c.DefaultQuery("limit", "50"))
	params.Offset, _ = strconv.Atoi(c.DefaultQuery("offset", "0"))

	// Реестр показывает ровно те проекты, которые пользователю видны.
	// Руководство и главный экономист видят все, остальные — назначенные
	// и созданные ими самими.
	if !h.acl.SeesAllProjects(claims, auth.PermProjectView) {
		ids := h.acl.VisibleProjectIDs(claims, auth.PermProjectView)
		if len(ids) == 0 {
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

	// Создатель получает доступ к своему проекту. Для инициатора это
	// вдобавок к авторству: без записи о назначении проект не попал бы
	// в чужие списки при передаче работы.
	_ = h.usersSvc.GrantProjectAccess(claims.UserID, p.ID, claims.UserID, true)

	h.audit.Log(auditlog.Entry{
		UserID: &claims.UserID, UserRole: claims.Role,
		Action: "create_project", ObjectType: "project", ObjectID: &p.ID,
		Comment: p.Name,
	})
	p.Permissions = h.acl.Permissions(claims, p.ID)
	c.JSON(http.StatusCreated, p)
}

func (h *Handler) get(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	claims := middleware.GetClaims(c)

	if !h.acl.Can(claims, auth.PermProjectView, id) {
		access.Deny(c, auth.PermProjectView)
		return
	}

	p, err := h.svc.Get(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "проект не найден"})
		return
	}
	// Что этот пользователь может делать с этим проектом. Фронт прячет
	// по этому списку кнопки; отказ всё равно выносит сервер.
	p.Permissions = h.acl.Permissions(claims, id)
	c.JSON(http.StatusOK, p)
}

func (h *Handler) update(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	claims := middleware.GetClaims(c)

	if !h.acl.Can(claims, auth.PermProjectEdit, id) {
		access.Deny(c, auth.PermProjectEdit)
		return
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
		Comment: p.Name,
	})
	p.Permissions = h.acl.Permissions(claims, id)
	c.JSON(http.StatusOK, p)
}

func (h *Handler) changeStatus(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	claims := middleware.GetClaims(c)

	if !h.acl.Can(claims, auth.PermProjectStatus, id) {
		access.Deny(c, auth.PermProjectStatus)
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
	p.Permissions = h.acl.Permissions(claims, id)
	c.JSON(http.StatusOK, p)
}
