package auth

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc   *Service
	limit *RateLimiter
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc, limit: NewRateLimiter()}
}

func (h *Handler) Register(r gin.IRouter) {
	r.POST("/auth/login", h.login)
}

func (h *Handler) login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Частота попыток проверяется до пароля: перебор должен упираться
	// в отказ, не доходя ни до сверки хеша, ни до счётчика блокировки
	// учётки (см. ratelimit.go).
	ip := c.ClientIP()
	email := strings.ToLower(strings.TrimSpace(req.Email))
	keys := []string{"ip:" + ip, "email:" + email}
	if ok, wait := h.limit.Allow(keys...); !ok {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error": fmt.Sprintf(
				"слишком много попыток входа. Повторите через %d с",
				int(wait.Seconds())+1),
		})
		return
	}

	resp, err := h.svc.Login(req)
	if err != nil {
		h.limit.Fail(keys...)
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	h.limit.Reset(keys...)
	c.JSON(http.StatusOK, resp)
}
