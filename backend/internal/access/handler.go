package access

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"ibcon-budget/internal/auth"
	"ibcon-budget/internal/middleware"
)

type Handler struct {
	acl *Service
}

func NewHandler(acl *Service) *Handler {
	return &Handler{acl: acl}
}

func (h *Handler) Register(r gin.IRouter) {
	// Справочник прав: коды, подписи и матрица ролей. Нужен экрану
	// управления пользователями, чтобы список выдаваемых прав и их
	// названия жили в одном месте — на сервере, рядом с проверкой.
	r.GET("/permissions", h.catalog)
	// Что может текущий пользователь в конкретном проекте. Отдельная
	// точка нужна редко: обычно права приходят вместе с карточкой
	// проекта и профилем.
	r.GET("/permissions/me", h.mine)
}

type permissionInfo struct {
	Code         string `json:"code"`
	Label        string `json:"label"`
	ProjectScope bool   `json:"project_scope"`
	Grantable    bool   `json:"grantable"`
	ReadOnly     bool   `json:"read_only"`
}

func (h *Handler) catalog(c *gin.Context) {
	perms := make([]permissionInfo, 0, len(auth.AllPermissions))
	for _, p := range auth.AllPermissions {
		perms = append(perms, permissionInfo{
			Code:         p,
			Label:        auth.PermissionLabels[p],
			ProjectScope: auth.IsProjectScoped(p),
			Grantable:    auth.IsGrantable(p),
			ReadOnly:     auth.IsReadOnly(p),
		})
	}

	// Матрица ролей в виде «роль → права, которые она даёт».
	roles := make([]gin.H, 0, len(auth.AllRoles))
	for _, r := range auth.AllRoles {
		roles = append(roles, gin.H{
			"code":        r,
			"label":       auth.RoleLabels[r],
			"permissions": auth.RolePermissions(r),
		})
	}

	c.JSON(http.StatusOK, gin.H{"permissions": perms, "roles": roles})
}

func (h *Handler) mine(c *gin.Context) {
	claims := middleware.GetClaims(c)
	projectID, _ := strconv.Atoi(c.Query("project_id"))
	c.JSON(http.StatusOK, gin.H{"permissions": h.acl.Permissions(claims, projectID)})
}
