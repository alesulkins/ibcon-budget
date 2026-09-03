package users

import (
	"net/http"
	"strconv"

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
	// Личный кабинет доступен любому авторизованному пользователю.
	// Регистрируем ДО группы /users, иначе «me» попал бы под проверку
	// права на управление пользователями.
	me := r.Group("/users/me")
	{
		me.GET("", h.getProfile)
		me.PUT("", h.updateProfile)
		me.PUT("/password", h.changeOwnPassword)
	}

	// Управление пользователями — по праву users.manage. Его даёт
	// только роль главного экономиста: индивидуально оно не выдаётся
	// (иначе получился бы второй ГЭ, см. auth.IsGrantable).
	g := r.Group("/users", h.acl.Require(auth.PermUsersManage))
	{
		g.GET("", h.list)
		g.POST("", h.create)
		g.GET("/:id", h.get)
		g.PUT("/:id", h.update)
		g.POST("/:id/password", h.setPassword)
		g.POST("/:id/unlock", h.unlock)
		g.POST("/:id/projects/:project_id/grant", h.grantAccess)
		g.DELETE("/:id/projects/:project_id/access", h.revokeAccess)
		// Множественный выбор проектов на экране управления: приходит
		// весь список сразу, лишнее отзывается.
		g.PUT("/:id/projects", h.setProjects)
		// Индивидуальные права сверх роли
		g.POST("/:id/permissions", h.addGrant)
		g.DELETE("/:id/permissions", h.removeGrant)
	}
}

func (h *Handler) list(c *gin.Context) {
	users, err := h.svc.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Доступы и права подтягиваем двумя запросами на весь список, а не
	// по запросу на строку таблицы.
	byUser, err := h.svc.ProjectAccessAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	grants, err := h.acl.GrantsForAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for i := range users {
		users[i].Projects = byUser[users[i].ID]
		users[i].Grants = toGrants(grants[users[i].ID])
	}
	c.JSON(http.StatusOK, users)
}

// toGrants переводит строки из пакета access в форму ответа.
func toGrants(rows []access.Grant) []Grant {
	out := make([]Grant, 0, len(rows))
	for _, r := range rows {
		out = append(out, Grant{
			Permission: r.Permission, ProjectID: r.ProjectID, ProjectName: r.ProjectName,
		})
	}
	return out
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
		Comment: u.FullName + " (" + u.Email + ")",
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
	u.Projects, _ = h.svc.ProjectAccessFor(id)
	if rows, gerr := h.acl.ListGrants(id); gerr == nil {
		u.Grants = toGrants(rows)
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
		Comment: u.FullName + " (" + u.Email + ")",
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
		// Кому выдали. Раньше писался голый id — по нему в журнале
		// невозможно было понять, о ком речь.
		Comment: h.userTitle(userID) + accessKind(body.CanEdit),
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
		Comment: h.userTitle(userID),
	})
	c.JSON(http.StatusOK, gin.H{"message": "доступ отозван"})
}

// setProjects приводит доступ пользователя к проектам к переданному списку.
func (h *Handler) setProjects(c *gin.Context) {
	userID, _ := strconv.Atoi(c.Param("id"))
	var req SetProjectsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cl := middleware.GetClaims(c)
	revoked, err := h.svc.SetProjectAccess(userID, req, cl.UserID)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	for _, pid := range revoked {
		_ = h.acl.RemoveGrantsForProject(userID, pid)
		h.audit.Log(auditlog.Entry{
			UserID: &cl.UserID, UserRole: cl.Role,
			Action: "revoke_project_access", ObjectType: "project", ObjectID: &pid,
			Comment: h.userTitle(userID),
		})
	}
	for _, it := range req.Projects {
		pid := it.ProjectID
		h.audit.Log(auditlog.Entry{
			UserID: &cl.UserID, UserRole: cl.Role,
			Action: "grant_project_access", ObjectType: "project", ObjectID: &pid,
			Comment: h.userTitle(userID) + accessKind(it.CanEdit),
		})
	}
	c.JSON(http.StatusOK, gin.H{"message": "доступ к проектам обновлён"})
}

func (h *Handler) addGrant(c *gin.Context) {
	userID, _ := strconv.Atoi(c.Param("id"))
	var req GrantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cl := middleware.GetClaims(c)
	if err := h.acl.AddGrant(userID, req.Permission, req.ProjectID, cl.UserID); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	h.audit.Log(auditlog.Entry{
		UserID: &cl.UserID, UserRole: cl.Role,
		Action: "grant_permission", ObjectType: "user", ObjectID: &userID,
		Comment: h.userTitle(userID) + " — " + permissionTitle(req.Permission, req.ProjectID),
	})
	c.JSON(http.StatusOK, gin.H{"message": "право выдано"})
}

func (h *Handler) removeGrant(c *gin.Context) {
	userID, _ := strconv.Atoi(c.Param("id"))
	var req GrantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cl := middleware.GetClaims(c)
	if err := h.acl.RemoveGrant(userID, req.Permission, req.ProjectID); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	h.audit.Log(auditlog.Entry{
		UserID: &cl.UserID, UserRole: cl.Role,
		Action: "revoke_permission", ObjectType: "user", ObjectID: &userID,
		Comment: h.userTitle(userID) + " — " + permissionTitle(req.Permission, req.ProjectID),
	})
	c.JSON(http.StatusOK, gin.H{"message": "право отозвано"})
}

// permissionTitle — как назвать выданное право в журнале.
func permissionTitle(perm string, projectID *int) string {
	name := auth.PermissionLabels[perm]
	if perm == auth.PermAll {
		name = "все права"
	}
	if name == "" {
		name = perm
	}
	if projectID == nil {
		return name + " (на все проекты)"
	}
	return name + " (проект №" + strconv.Itoa(*projectID) + ")"
}

// ─── Личный кабинет ─────────────────────────────────────────────────────

func (h *Handler) getProfile(c *gin.Context) {
	claims := middleware.GetClaims(c)
	p, err := h.svc.GetProfile(claims.UserID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	// Права отдаём вместе с профилем: фронт запрашивает его при входе
	// и по ним прячет недоступные разделы.
	p.Permissions = h.acl.Permissions(claims, 0)
	c.JSON(http.StatusOK, p)
}

func (h *Handler) updateProfile(c *gin.Context) {
	claims := middleware.GetClaims(c)
	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	p, err := h.svc.UpdateProfile(claims.UserID, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *Handler) changeOwnPassword(c *gin.Context) {
	claims := middleware.GetClaims(c)
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.ChangeOwnPassword(claims.UserID, req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.audit.Log(auditlog.Entry{
		UserID:     &claims.UserID,
		UserRole:   claims.Role,
		Action:     "change_own_password",
		ObjectType: "user",
		ObjectID:   &claims.UserID,
		Comment:    "Пользователь сменил свой пароль",
	})

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// userTitle — как назвать пользователя в журнале: «Иванов И.И. (mail)».
// Если запись не читается, остаётся хотя бы номер.
func (h *Handler) userTitle(id int) string {
	u, err := h.svc.Get(id)
	if err != nil || u == nil {
		return "пользователь №" + strconv.Itoa(id)
	}
	return u.FullName + " (" + u.Email + ")"
}

// accessKind — с правом правки или только на чтение.
func accessKind(canEdit bool) string {
	if canEdit {
		return " — с правом редактирования"
	}
	return " — только просмотр"
}
