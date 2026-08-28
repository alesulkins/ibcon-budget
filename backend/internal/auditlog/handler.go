package auditlog

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"ibcon-budget/internal/access"
	"ibcon-budget/internal/auth"
	"ibcon-budget/internal/middleware"
)

type Handler struct {
	svc *Service
	acl *access.Service
}

func NewHandler(svc *Service, acl *access.Service) *Handler {
	return &Handler{svc: svc, acl: acl}
}

func (h *Handler) Register(r gin.IRouter) {
	r.GET("/audit-log", h.acl.Require(auth.PermAuditView), h.list)
}

func (h *Handler) list(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	params := ListParams{Limit: limit, Offset: offset}

	// Журнал сквозной по всей системе, поэтому тем, кто видит не все
	// проекты, он ограничивается их проектами. Иначе история выдавала бы
	// названия и суммы чужих проектов тому, кому сами проекты закрыты.
	claims := middleware.GetClaims(c)
	if !h.acl.SeesAllProjects(claims, auth.PermAuditView) {
		ids := h.acl.VisibleProjectIDs(claims, auth.PermAuditView)
		if len(ids) == 0 {
			c.JSON(http.StatusOK, gin.H{"total": 0, "items": []LogRow{}})
			return
		}
		params.ProjectIDs = ids
	}

	rows, total, err := h.svc.List(params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"total": total, "items": rows})
}
