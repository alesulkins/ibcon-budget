package users

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"ibcon-budget/internal/auth"
	"ibcon-budget/internal/auditlog"
	"ibcon-budget/internal/middleware"
)

type Handler struct {
	svc   *Service
	audit *auditlog.Service
}

func NewHandler(svc *Service, audit *auditlog.Service) *Handler {
	return &Handler{svc: svc, audit: audit}
}

func (h *Handler) Register(r gin.IRouter) {
	g := r.Group("/users", middleware.RequireRole(auth.RoleGE))
	{
		g.GET("", h.list)
		g.POST("", h.create)
		g.GET("/:id", h.get)
		g.PUT("/:id", h.update)
		g.POST("/:id/password", h.setPassword)
		g.POST("/:id/unlock", h.unlock)
		g.POST("/:id/projects/:project_id/grant", h.grantAccess)
		g.DELETE("/:id/projects/:project_id/access", h.revokeAccess)
	}
}

func (h *Handler) list(c *gin.Context) {
	users, err := h.svc.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, users)
}

func (h *Handler) create(c *gin.Context) {
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cl := middleware.GetClaims(c)
	u, err := h.svc.Create(req, cl.UserID)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	h.audit.Log(auditlog.Entry{
		UserID: &cl.UserID, UserRole: cl.Role,
		Action: "create_user", ObjectType: "user", ObjectID: &u.ID,
	})
	c.JSON(http.StatusCreated, u)
}

func (h *Handler) get(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	u, err := h.svc.Get(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "пользователь не найден"})
		return
	}
	c.JSON(http.StatusOK, u)
}

func (h *Handler) update(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	u, err := h.svc.Update(id, req)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	cl := middleware.GetClaims(c)
	h.audit.Log(auditlog.Entry{
		UserID: &cl.UserID, UserRole: cl.Role,
		Action: "update_user", ObjectType: "user", ObjectID: &u.ID,
	})
	c.JSON(http.StatusOK, u)
}

func (h *Handler) setPassword(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req SetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.SetPassword(id, req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "пароль обновлён"})
}

func (h *Handler) unlock(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.svc.UnlockUser(id); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "блокировка снята"})
}

func (h *Handler) grantAccess(c *gin.Context) {
	userID, _ := strconv.Atoi(c.Param("id"))
	projectID, _ := strconv.Atoi(c.Param("project_id"))
	var body struct {
		CanEdit bool `json:"can_edit"`
	}
	_ = c.ShouldBindJSON(&body)
	cl := middleware.GetClaims(c)
	if err := h.svc.GrantProjectAccess(userID, projectID, cl.UserID, body.CanEdit); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	h.audit.Log(auditlog.Entry{
		UserID: &cl.UserID, UserRole: cl.Role,
		Action: "grant_project_access", ObjectType: "project", ObjectID: &projectID,
		Comment: strconv.Itoa(userID),
	})
	c.JSON(http.StatusOK, gin.H{"message": "доступ выдан"})
}

func (h *Handler) revokeAccess(c *gin.Context) {
	userID, _ := strconv.Atoi(c.Param("id"))
	projectID, _ := strconv.Atoi(c.Param("project_id"))
	cl := middleware.GetClaims(c)
	if err := h.svc.RevokeProjectAccess(userID, projectID); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	h.audit.Log(auditlog.Entry{
		UserID: &cl.UserID, UserRole: cl.Role,
		Action: "revoke_project_access", ObjectType: "project", ObjectID: &projectID,
		Comment: strconv.Itoa(userID),
	})
	c.JSON(http.StatusOK, gin.H{"message": "доступ отозван"})
}
