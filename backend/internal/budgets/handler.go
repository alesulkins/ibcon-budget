package budgets

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"

	"ibcon-budget/internal/access"
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
	acl         *access.Service
	audit       *auditlog.Service
}

func NewHandler(svc *Service, projectsSvc *projects.Service, usersSvc *users.Service,
	acl *access.Service, audit *auditlog.Service) *Handler {
	return &Handler{svc: svc, projectsSvc: projectsSvc, usersSvc: usersSvc, acl: acl, audit: audit}
}

// Права проверяются внутри обработчиков, а не посредником по ролям:
// каждому нужен id проекта, чтобы отличить «может вообще» от «может
// в этом проекте». Посредник по ролям такого различия не делал и
// пропускал, например, экономиста в чужой бюджет.
func (h *Handler) Register(r gin.IRouter) {
	// Маршруты на уровне проекта (параметр :id = project_id)
	proj := r.Group("/projects/:id/budgets")
	proj.GET("/versions", h.listVersions)
	proj.POST("/versions", h.createVersion)
	proj.POST("/new-version", h.newVersion)

	// Маршруты на уровне версии бюджета (параметр :vid = version_id)
	ver := r.Group("/budget-versions/:vid")
	ver.GET("", h.getVersion)
	ver.PATCH("/status", h.changeStatus)
	ver.PUT("/cost-override", h.updateCostOverride)
	ver.PUT("/inputs/:type", h.saveInput)
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

// canEditVersion — можно ли править данные конкретной версии.
//
// Право «изменение версии бюджета» в этом проекте даёт матрица доступа.
// Сверх него действует правило владельца 2026-08-27: согласованную и
// архивную версию нельзя менять напрямую — исключение сделано для
// АВТОРА версии и главного экономиста. Остальным остаётся создать новую
// версию копированием.
func (h *Handler) canEditVersion(claims *auth.Claims, v *BudgetVersion) (bool, string) {
	if !h.acl.Can(claims, auth.PermBudgetEdit, v.ProjectID) {
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

// createPerm — какое право нужно для создания версии в этом проекте.
// Первая версия и последующие разведены таблицей 1 ТЗ: экономист
// проекта создаёт новые версии, но не первую.
func (h *Handler) createPerm(projectID int) string {
	existing, err := h.svc.ListVersions(projectID)
	if err != nil || len(existing) == 0 {
		return auth.PermBudgetCreate
	}
	return auth.PermBudgetVersion
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
	if !h.acl.Can(claims, auth.PermBudgetView, pid) {
		access.Deny(c, auth.PermBudgetView)
		return
	}
	versions, err := h.svc.ListVersions(pid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Что этот пользователь может делать с бюджетами проекта — фронт
	// прячет по этому списку кнопки создания версии, смены статуса и
	// выгрузки.
	perms := h.acl.Permissions(claims, pid)
	for i := range versions {
		versions[i].Permissions = perms
	}
	c.JSON(http.StatusOK, versions)
}

func (h *Handler) createVersion(c *gin.Context) {
	pid := h.projectID(c)
	claims := middleware.GetClaims(c)

	// Первая версия и новая версия — разные права (таблица 1 ТЗ).
	// Какое из них требуется, решает наличие версий, а не то, какой
	// маршрут выбрал фронт: иначе прямой запрос на /versions обошёл бы
	// запрет создавать первую версию.
	if perm := h.createPerm(pid); !h.acl.Can(claims, perm, pid) {
		access.Deny(c, perm)
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

	if perm := h.createPerm(pid); !h.acl.Can(claims, perm, pid) {
		access.Deny(c, perm)
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
	if !h.acl.Can(claims, auth.PermBudgetView, v.ProjectID) {
		access.Deny(c, auth.PermBudgetView)
		return
	}
	v.Permissions = h.acl.Permissions(claims, v.ProjectID)
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

	if !h.acl.Can(claims, auth.PermBudgetStatus, v.ProjectID) {
		access.Deny(c, auth.PermBudgetStatus)
		return
	}

	// Согласование — отдельное право
	var req ChangeStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Status == StatusApproved && !h.acl.Can(claims, auth.PermBudgetApprove, v.ProjectID) {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "статус «Согласован» ставит главный экономист",
			"code":  "forbidden",
		})
		return
	}

	// Чужую версию не трогаем: менять статус может её автор, главный
	// экономист и тот, кому право выдано индивидуально.
	if !CanEditFrozenVersion(claims.Role, claims.UserID, v.CreatedBy) &&
		!h.acl.Can(claims, auth.PermBudgetApprove, v.ProjectID) {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "менять статус версии может её автор или главный экономист",
			"code":  "forbidden",
		})
		return
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
	if !h.acl.Can(claims, auth.PermBudgetView, v.ProjectID) {
		access.Deny(c, auth.PermBudgetView)
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
	if !h.acl.Can(claims, auth.PermBudgetView, v.ProjectID) {
		access.Deny(c, auth.PermBudgetView)
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
	if !h.acl.Can(claims, auth.PermBudgetView, v.ProjectID) {
		access.Deny(c, auth.PermBudgetView)
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
	if !h.acl.Can(claims, auth.PermBudgetExport, v.ProjectID) {
		access.Deny(c, auth.PermBudgetExport)
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
	// Администратору проекта книга собирается урезанной — строки
	// 178-214 листа «2.Бюджет», без ФОТ, выручки и прибыли.
	meta.Limited = auth.LimitedExport(claims.Role)

	data, err := BuildExport(meta, result)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "не удалось собрать файл: " + err.Error()})
		return
	}

	h.audit.Log(auditlog.Entry{
		UserID: &claims.UserID, UserRole: claims.Role,
		Action: "export_budget", ObjectType: "budget_version", ObjectID: &vid,
		Comment: exportComment(proj.ID, v.VersionNo, proj.Name, meta.Limited),
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

// exportComment — что записать в журнал о выгрузке. Урезанную выгрузку
// администратора проекта помечаем: иначе по журналу не отличить её от
// полной книги.
func exportComment(projectID, versionNo int, name string, limited bool) string {
	s := fmt.Sprintf("Бюджет %d.%d — %s", projectID, versionNo, name)
	if limited {
		s += " (выгрузка администратора проекта: строки 178-214)"
	}
	return s
}
