package reminders

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"ibcon-budget/internal/middleware"
)

// Напоминания — личные: их видит и правит только владелец, поэтому
// отдельных прав здесь нет, а id пользователя всегда берётся из токена,
// а не из запроса.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(r gin.IRouter) {
	g := r.Group("/reminders")
	g.GET("", h.list)
	g.POST("", h.create)
	g.PUT("/:id", h.update)
	g.DELETE("/:id", h.remove)
	// Наступившие: фронт спрашивает их периодически и показывает
	// уведомлением, затем подтверждает показ.
	g.GET("/due", h.due)
	g.POST("/shown", h.markShown)
}

func userID(c *gin.Context) int { return middleware.GetClaims(c).UserID }

func (h *Handler) list(c *gin.Context) {
	rows, err := h.svc.List(userID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (h *Handler) create(c *gin.Context) {
	var body struct {
		Text     string    `json:"text"      binding:"required"`
		RemindAt time.Time `json:"remind_at" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	r, err := h.svc.Create(userID(c), body.Text, body.RemindAt)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, r)
}

func (h *Handler) update(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		Text     *string    `json:"text"`
		RemindAt *time.Time `json:"remind_at"`
		Done     *bool      `json:"done"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	r, err := h.svc.Update(userID(c), id, body.Text, body.RemindAt, body.Done)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, r)
}

func (h *Handler) remove(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.svc.Delete(userID(c), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) due(c *gin.Context) {
	rows, err := h.svc.Due(userID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (h *Handler) markShown(c *gin.Context) {
	var body struct {
		IDs []int `json:"ids"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.MarkShown(userID(c), body.IDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
