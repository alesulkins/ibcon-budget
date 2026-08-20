package main

import (
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"

	"ibcon-budget/internal/auditlog"
	"ibcon-budget/internal/auth"
	"ibcon-budget/internal/budgets"
	"ibcon-budget/internal/db"
	"ibcon-budget/internal/middleware"
	"ibcon-budget/internal/projects"
	"ibcon-budget/internal/references"
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
	r.Use(middleware.CORS())

	// Публичные маршруты
	api := r.Group("/api/v1")
	authSvc := auth.NewService(database, cfg.JWTSecret, cfg.JWTExpiryHours)
	auth.NewHandler(authSvc).Register(api)

	// Защищённые маршруты
	protected := api.Group("", middleware.Auth(cfg.JWTSecret, database))

	auditSvc := auditlog.NewService(database)
	auditlog.NewHandler(auditSvc).Register(protected)

	usersSvc := users.NewService(database)
	users.NewHandler(usersSvc, auditSvc).Register(protected)

	refSvc := references.NewService(database)
	references.NewHandler(refSvc, auditSvc).Register(protected)

	projectsSvc := projects.NewService(database)
	projects.NewHandler(projectsSvc, usersSvc, auditSvc).Register(protected)

	budgetsSvc := budgets.NewService(database)
	budgets.NewHandler(budgetsSvc, projectsSvc, usersSvc, auditSvc).Register(protected)

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
