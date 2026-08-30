package main

import (
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"

	"ibcon-budget/internal/access"
	"ibcon-budget/internal/auditlog"
	"ibcon-budget/internal/auth"
	"ibcon-budget/internal/budgets"
	"ibcon-budget/internal/db"
	"ibcon-budget/internal/middleware"
	"ibcon-budget/internal/projects"
	"ibcon-budget/internal/references"
	"ibcon-budget/internal/reminders"
	"ibcon-budget/internal/users"
	"ibcon-budget/pkg/config"
	"ibcon-budget/pkg/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "err", err)
		os.Exit(1)
	}

	logger.Init(os.Getenv("GIN_MODE"))

	if err := db.RunMigrations(cfg); err != nil {
		slog.Error("migrations failed", "err", err)
		os.Exit(1)
	}

	database, err := db.Connect(cfg)
	if err != nil {
		slog.Error("db connect failed", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	gin.SetMode(os.Getenv("GIN_MODE"))
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORS(cfg.AllowedOrigins))

	// Публичные маршруты
	api := r.Group("/api/v1")
	authSvc := auth.NewService(database, cfg.JWTSecret, cfg.JWTExpiryHours)
	auth.NewHandler(authSvc).Register(api)

	// Защищённые маршруты
	protected := api.Group("", middleware.Auth(cfg.JWTSecret, database))

	auditSvc := auditlog.NewService(database)

	// Проверка прав. Один сервис на все модули: матрица ролей и
	// индивидуальные права должны трактоваться одинаково везде.
	acl := access.NewService(database)
	access.NewHandler(acl).Register(protected)

	auditlog.NewHandler(auditSvc, acl).Register(protected)

	usersSvc := users.NewService(database)
	users.NewHandler(usersSvc, acl, auditSvc).Register(protected)

	refSvc := references.NewService(database)
	references.NewHandler(refSvc, acl, auditSvc).Register(protected)

	// Напоминания личного кабинета. Показываются всплывающим
	// уведомлением на экране; рассылки писем нет — решение владельца
	// 2026-08-30.
	remSvc := reminders.NewService(database)
	reminders.NewHandler(remSvc).Register(protected)

	projectsSvc := projects.NewService(database)
	projects.NewHandler(projectsSvc, usersSvc, acl, auditSvc).Register(protected)

	budgetsSvc := budgets.NewService(database)
	budgets.NewHandler(budgetsSvc, projectsSvc, usersSvc, refSvc, acl, auditSvc).Register(protected)

	// Health check
	r.GET("/health", func(c *gin.Context) {
		if err := database.Ping(); err != nil {
			c.JSON(503, gin.H{"status": "unhealthy", "error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "ok"})
	})

	addr := ":" + cfg.ServerPort
	slog.Info("server starting", "addr", addr)
	if err := r.Run(addr); err != nil {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}
