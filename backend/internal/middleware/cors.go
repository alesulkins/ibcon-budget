package middleware

import (
	"github.com/gin-gonic/gin"
)

// CORS разрешает обращения только с заданных адресов. Раньше здесь стояла
// звёздочка — API отвечал кому угодно.
func CORS(allowed []string) gin.HandlerFunc {
	index := make(map[string]bool, len(allowed))
	for _, o := range allowed {
		index[o] = true
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		// Отвечаем ровно тем адресом, который спросил, и только если он
		// в списке: перечислять несколько адресов в заголовке нельзя.
		if origin != "" && index[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
			// Ответ зависит от Origin — без этого промежуточный кэш
			// отдаст чужому сайту разрешение, выданное разрешённому.
			c.Header("Vary", "Origin")
		}
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
