package web

import (
    "context"
    "fmt"
    "net/http"
    "time"

    "github.com/casapps/casspaces/internal/database"
    "github.com/gin-gonic/gin"
    "github.com/sirupsen/logrus"
)

type Server struct {
    config *Config
    router *gin.Engine
    server *http.Server
    logger *logrus.Logger
}

type Config struct {
    Database     database.Database
    Auth         interface{}
    Security     interface{}
    Workspace    interface{}
    Monitoring   interface{}
    Backup       interface{}
    Cloud        interface{}
    Logger       *logrus.Logger
}

func New(config *Config) (*Server, error) {
    gin.SetMode(gin.ReleaseMode)
    router := gin.New()

    // Basic routes
    router.GET("/", func(c *gin.Context) {
        c.JSON(200, gin.H{
            "message": "CasjayDev Workspaces",
            "status":  "running",
            "version": "1.0.0",
        })
    })

    router.GET("/health", func(c *gin.Context) {
        c.JSON(200, gin.H{"status": "healthy"})
    })

    // API routes
    api := router.Group("/api/v1")
    {
        api.GET("/status", func(c *gin.Context) {
            c.JSON(200, gin.H{
                "status": "operational",
                "services": gin.H{
                    "auth":      "active",
                    "security":  "active",
                    "workspace": "active",
                },
            })
        })
    }

    return &Server{
        config: config,
        router: router,
        logger: config.Logger,
    }, nil
}

func (s *Server) Start(ctx context.Context) error {
    port := 8080

    s.server = &http.Server{
        Addr:    fmt.Sprintf(":%d", port),
        Handler: s.router,
    }

    s.logger.Infof("🌐 Web server starting on port %d", port)

    go func() {
        if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            s.logger.WithError(err).Error("Web server failed")
        }
    }()

    s.logger.Info("✅ Web server started")
    return nil
}

func (s *Server) Stop() {
    if s.server != nil {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        s.server.Shutdown(ctx)
    }
    s.logger.Info("✅ Web server stopped")
}

func NewSetupServer(db database.Database, logger *logrus.Logger) (*Server, error) {
    gin.SetMode(gin.ReleaseMode)
    router := gin.New()

    router.GET("/setup", func(c *gin.Context) {
        c.JSON(200, gin.H{
            "message": "Setup wizard",
            "step":    "welcome",
        })
    })

    return &Server{
        router: router,
        logger: logger,
    }, nil
}

func (s *Server) StartSetup(ctx context.Context, port int) error {
    s.server = &http.Server{
        Addr:    fmt.Sprintf(":%d", port),
        Handler: s.router,
    }

    go func() {
        if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            s.logger.WithError(err).Error("Setup server failed")
        }
    }()

    <-ctx.Done()
    s.Stop()
    return nil
}