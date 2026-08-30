package middleware

import (
	"github.com/gin-gonic/gin"
)

// CORS разрешает обращения только с заданных адресов.
//
// Раньше здесь стояла звёздочка — API отвечал кому угодно. Сейчас
// браузер этим воспользоваться не мог (токен идёт заголовком, а не
// куками, и cookie-авторизации нет), но звёздочка снимает защиту
// заранее: стоит однажды перевести вход на куки, и любой сайт начнёт
// ходить в API от имени открывшего его человека.
//
// Список задаётся переменной ALLOWED_ORIGINS через запятую. Пустой
// список — обычный рабочий случай: и в разработке (Vite проксирует
// /api на бэкенд), и на стенде фронтенд отдаётся с того же адреса, а
// запросу на свой же адрес заголовки CORS не нужны вовсе.
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
