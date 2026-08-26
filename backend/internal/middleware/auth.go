package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"

	"ibcon-budget/internal/auth"
)

const (
	ClaimsKey    = "claims"
	sessionTimeout = 60 * time.Minute
)

func Auth(jwtSecret string, db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "требуется авторизация"})
			return
		}
		tokenStr := strings.TrimPrefix(header, "Bearer ")
		claims, err := auth.ParseToken(tokenStr, jwtSecret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "невалидный токен"})
			return
		}

		// Учётка должна существовать и быть активной. Раньше ошибка запроса
		// молча игнорировалась, поэтому отключённый пользователь продолжал
		// работать с уже выданным токеном до истечения его срока.
		var lastActivity *time.Time
		err = db.Get(&lastActivity, `SELECT last_activity FROM users WHERE id=$1 AND active=TRUE`, claims.UserID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "учётная запись отключена или не найдена",
				"code":  "account_disabled",
			})
			return
		}

		// Автовыход по бездействию (ТЗ 3.8 п.6): 60 минут без запросов.
		// Действует независимо от «Запомнить меня» — тот управляет только
		// сроком жизни токена.
		if lastActivity != nil && time.Since(*lastActivity) > sessionTimeout {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "сессия истекла из-за бездействия, войдите снова",
				"code":  "session_timeout",
			})
			return
		}

		// Обновляем last_activity асинхронно
		go func() {
			_, _ = db.Exec(`UPDATE users SET last_activity=NOW() WHERE id=$1`, claims.UserID)
		}()

		c.Set(ClaimsKey, claims)
		c.Next()
	}
}

func GetClaims(c *gin.Context) *auth.Claims {
	v, _ := c.Get(ClaimsKey)
	claims, _ := v.(*auth.Claims)
	return claims
}

// RequireRole возвращает 403 если у пользователя нет одной из указанных ролей
func RequireRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(c *gin.Context) {
		claims := GetClaims(c)
		if claims == nil || !allowed[claims.Role] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "недостаточно прав"})
			return
		}
		c.Next()
	}
}
