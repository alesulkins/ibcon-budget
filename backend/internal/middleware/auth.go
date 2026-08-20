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

		// Проверка сессионного таймаута по last_activity
		var lastActivity *time.Time
		_ = db.Get(&lastActivity, `SELECT last_activity FROM users WHERE id=$1 AND active=TRUE`, claims.UserID)
		if lastActivity != nil && time.Since(*lastActivity) > sessionTimeout {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "сессия истекла, войдите снова"})
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
