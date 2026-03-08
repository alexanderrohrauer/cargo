package api

import (
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"

	"cargo/internal/api/handlers"
	"cargo/internal/api/middleware"
	"cargo/internal/config"
	"cargo/internal/project"
	"cargo/internal/ssl"
)

// Server is the Cargo REST API server.
type Server struct {
	Manager   *project.Manager
	AuthToken string
	Config    *config.Config
}

// NewServer creates a new Server instance.
func NewServer(mgr *project.Manager, token string, cfg *config.Config) *Server {
	return &Server{
		Manager:   mgr,
		AuthToken: token,
		Config:    cfg,
	}
}

// Run starts the HTTPS server and blocks until it exits.
func (s *Server) Run() error {
	tlsCfg, err := ssl.LoadTLSConfig(s.Config.Workdir)
	if err != nil {
		return fmt.Errorf("loading TLS config: %w", err)
	}
	_ = tlsCfg // gin uses cert/key file paths directly

	router := s.setupRoutes()

	addr := fmt.Sprintf("%s:%d", s.Config.Server.Host, s.Config.Server.Port)
	slog.Info("starting HTTPS server",
		"addr", addr,
		"cert", ssl.CertPath(s.Config.Workdir),
	)

	certPath := ssl.CertPath(s.Config.Workdir)
	keyPath := ssl.KeyPath(s.Config.Workdir)

	return router.RunTLS(addr, certPath, keyPath)
}

// setupRoutes configures the Gin router with all API routes and middleware.
func (s *Server) setupRoutes() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(ginSlogLogger())

	// Public route
	router.GET("/api/v1/health", handlers.HealthCheck)

	// Authenticated routes
	authed := router.Group("/api/v1")
	authed.Use(middleware.AuthToken(s.AuthToken))
	{
		authed.GET("/projects", handlers.ListProjects(s.Manager))
		authed.POST("/projects/sync", handlers.SyncAllProjects(s.Manager))
		authed.POST("/projects/:name/sync", handlers.SyncProject(s.Manager))
		authed.GET("/projects/:name/status", handlers.GetProjectStatus(s.Manager))
	}

	return router
}

// ginSlogLogger returns a Gin middleware that logs requests using slog.
func ginSlogLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		slog.Info("request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"client_ip", c.ClientIP(),
		)
	}
}
