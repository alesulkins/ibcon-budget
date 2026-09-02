package market

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler — один маршрут: спросить у платформы рыночную стоимость
// аренды. Прав на него отдельных нет: это справочный запрос, он ничего
// не меняет и не привязан к проекту, — но маршрут в защищённой группе,
// чужой к нему не обратится.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(r gin.IRouter) {
	r.POST("/market/rent-estimate", h.estimate)
	// Пояснения к расчёту и вопрос ассистенту. Методика отдаётся всегда,
	// ответ ассистента — только если подключён ключ к модели.
	r.GET("/market/methodology", h.methodology)
	r.POST("/market/ask", h.ask)
}

func (h *Handler) methodology(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"blocks": MethodologyText()})
}

func (h *Handler) ask(c *gin.Context) {
	var req ExplainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "вопрос не разобран"})
		return
	}
	answer, err := Ask(c.Request.Context(), req)
	if errors.Is(err, ErrNoAssistant) {
		// 501, а не 500: платформа исправна, просто эта возможность не
		// включена — на экране это разные сообщения.
		c.JSON(http.StatusNotImplemented, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"answer": answer})
}

func (h *Handler) estimate(c *gin.Context) {
	var q RentQuery
	if err := c.ShouldBindJSON(&q); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "город обязателен"})
		return
	}
	est, err := h.svc.Estimate(c.Request.Context(), q)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, est)
}
