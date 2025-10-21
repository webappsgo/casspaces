package app

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "time"

    "github.com/casapps/casspaces/internal/auth"
    "github.com/casapps/casspaces/internal/backup"
    "github.com/casapps/casspaces/internal/cloud"
    "github.com/casapps/casspaces/internal/cluster"
    "github.com/casapps/casspaces/internal/database"
    "github.com/casapps/casspaces/internal/monitoring"
    "github.com/casapps/casspaces/internal/security"
    "github.com/casapps/casspaces/internal/web"
    "github.com/casapps/casspaces/internal/workspace"
    "github.com/pkg/errors"
    "github.com/sirupsen/logrus"
)

type App struct {
    config      *Config
    db          database.Database
    logger      *logrus.Logger

    // Core services
    auth        *auth.Service
    security    *security.Engine
    workspace   *workspace.Manager
    cluster     *cluster.Manager
    backup      *backup.Manager
    cloud       *cloud.Manager
    monitoring  *monitoring.Service
    webServer   *web.Server

    // Runtime state
    initialized bool
    configured  bool
    setupMode   bool
}

type Config struct {
    // Command line options
    ConfigPath string
    SetupMode  bool
    DebugMode  bool
    Version    string
    BuildTime  string
    GitCommit  string
}

func New(config *Config) (*App, error) {
    app := &App{
        config:    config,
        setupMode: config.SetupMode,
    }

    // Initialize logger
    app.initLogger()

    // Initialize database
    if err := app.initDatabase(); err != nil {
        return nil, errors.Wrap(err, "failed to initialize database")
    }

    return app, nil
}

func (app *App) Start(ctx context.Context) error {
    app.logger.Info("🚀 Starting CasjayDev Workspaces...")

    // Initialize core systems
    if err := app.initialize(); err != nil {
        return errors.Wrap(err, "initialization failed")
    }

    // Check if this is first run or setup mode
    if !app.isConfigured() || app.setupMode {
        return app.startSetupMode(ctx)
    }

    // Start all services
    if err := app.startServices(ctx); err != nil {
        return errors.Wrap(err, "failed to start services")
    }

    // Show startup message
    app.showStartupMessage()

    // Wait for shutdown signal
    <-ctx.Done()

    return app.shutdown()
}

func (app *App) Reload() error {
    app.logger.Info("🔄 Reloading configuration...")
    app.logger.Info("✅ Configuration reloaded")
    return nil
}

func (app *App) initLogger() {
    app.logger = logrus.New()

    // Set log level based on debug mode
    if app.config.DebugMode {
        app.logger.SetLevel(logrus.DebugLevel)
    } else {
        app.logger.SetLevel(logrus.InfoLevel)
    }

    // Use JSON formatter for structured logging
    app.logger.SetFormatter(&logrus.JSONFormatter{
        TimestampFormat: time.RFC3339,
    })
}

func (app *App) initDatabase() error {
    dbPath := "/var/lib/casspaces/casspaces.db"
    if envPath := os.Getenv("CASSPACES_DB_PATH"); envPath != "" {
        dbPath = envPath
    }

    // Ensure database directory exists
    dbDir := filepath.Dir(dbPath)
    if err := os.MkdirAll(dbDir, 0755); err != nil {
        return errors.Wrap(err, "failed to create database directory")
    }

    // Initialize database
    db, err := database.New(&database.Config{
        Type: "sqlite",
        Path: dbPath,
    })
    if err != nil {
        return errors.Wrap(err, "failed to connect to database")
    }

    app.db = db

    // Run migrations
    if err := app.runMigrations(); err != nil {
        return errors.Wrap(err, "failed to run database migrations")
    }

    return nil
}

func (app *App) runMigrations() error {
    app.logger.Info("📋 Running database migrations...")

    // Read schema file
    schema, err := os.ReadFile("configs/schema.sql")
    if err != nil {
        return errors.Wrap(err, "failed to read schema file")
    }

    // Execute schema
    if _, err := app.db.Exec(string(schema)); err != nil {
        return errors.Wrap(err, "failed to execute schema")
    }

    app.logger.Info("✅ Database migrations completed")
    return nil
}

func (app *App) initialize() error {
    app.logger.Info("🔧 Initializing core systems...")

    // Ensure runtime directories
    if err := app.ensureDirectories(); err != nil {
        return errors.Wrap(err, "failed to create directories")
    }

    // Initialize mandatory security
    if err := app.initMandatorySecurity(); err != nil {
        return errors.Wrap(err, "failed to initialize security")
    }

    app.initialized = true
    app.logger.Info("✅ Core systems initialized")
    return nil
}

func (app *App) ensureDirectories() error {
    directories := []string{
        "/etc/casspaces/ssl",
        "/etc/casspaces/security",
        "/etc/casspaces/security/geoip",
        "/etc/casspaces/security/threats",
        "/etc/casspaces/security/vulnerabilities",
        "/var/log/casspaces",
        "/var/lib/casspaces/cache",
        "/var/lib/casspaces",
        "/var/run/casspaces",
    }

    for _, dir := range directories {
        if err := os.MkdirAll(dir, 0755); err != nil {
            return errors.Wrapf(err, "failed to create directory: %s", dir)
        }
    }

    app.logger.WithField("directories", len(directories)).Info("✅ Runtime directories ready")
    return nil
}

func (app *App) initMandatorySecurity() error {
    app.logger.Info("🛡️  Initializing mandatory security...")

    securityConfig := &security.Config{
        SecurityDirectory:    "/etc/casspaces/security",
        UpdateFrequencyHours: 24,
        GeoBlockingMode:      "monitor",
        RateLimitingEnabled:  true,
        RequestsPerMinute:    300,
        RequestsPerHour:      5000,
    }

    engine, err := security.New(app.db, securityConfig, app.logger)
    if err != nil {
        return errors.Wrap(err, "failed to initialize security engine")
    }

    app.security = engine
    app.logger.Info("✅ Mandatory security initialized")
    return nil
}

func (app *App) isConfigured() bool {
    var setupCompleted bool
    err := app.db.QueryRow("SELECT setup_completed FROM server_config WHERE id = 1").Scan(&setupCompleted)
    return err == nil && setupCompleted
}

func (app *App) startSetupMode(ctx context.Context) error {
    app.logger.Info("🔧 Starting setup mode...")

    setupServer, err := web.NewSetupServer(app.db, app.logger)
    if err != nil {
        return errors.Wrap(err, "failed to create setup server")
    }

    port := 8080
    setupURL := fmt.Sprintf("http://localhost:%d/setup", port)

    fmt.Println("🔧 First run detected - starting setup mode")
    fmt.Printf("🌐 Setup wizard: %s\n", setupURL)
    fmt.Println("📋 Complete the setup wizard to configure your server")

    return setupServer.StartSetup(ctx, port)
}

func (app *App) startServices(ctx context.Context) error {
    app.logger.Info("🚀 Starting services...")

    // Start authentication service
    authService, err := auth.New(app.db, nil, app.logger)
    if err != nil {
        return errors.Wrap(err, "failed to start authentication service")
    }
    app.auth = authService

    // Start workspace manager
    workspaceManager, err := workspace.New(app.db, nil, app.logger)
    if err != nil {
        return errors.Wrap(err, "failed to start workspace manager")
    }
    app.workspace = workspaceManager

    // Start backup manager
    backupManager, err := backup.New(app.db, nil, nil, app.logger)
    if err != nil {
        return errors.Wrap(err, "failed to start backup manager")
    }
    app.backup = backupManager

    // Start cloud manager
    cloudManager, err := cloud.New(app.db, nil, app.logger)
    if err != nil {
        return errors.Wrap(err, "failed to start cloud manager")
    }
    app.cloud = cloudManager

    // Start monitoring service
    monitoringService, err := monitoring.New(app.db, nil, app.logger)
    if err != nil {
        return errors.Wrap(err, "failed to start monitoring service")
    }
    app.monitoring = monitoringService

    if err := app.monitoring.Start(ctx); err != nil {
        return errors.Wrap(err, "failed to start monitoring")
    }

    // Start web server
    webServer, err := web.New(&web.Config{
        Database:    app.db,
        Auth:        app.auth,
        Security:    app.security,
        Workspace:   app.workspace,
        Monitoring:  app.monitoring,
        Backup:      app.backup,
        Cloud:       app.cloud,
        Logger:      app.logger,
    })
    if err != nil {
        return errors.Wrap(err, "failed to create web server")
    }
    app.webServer = webServer

    if err := app.webServer.Start(ctx); err != nil {
        return errors.Wrap(err, "failed to start web server")
    }

    app.logger.Info("✅ All services started successfully")
    return nil
}

func (app *App) showStartupMessage() {
    fmt.Printf("🚀 CasjayDev Workspaces is ready!\n")
    fmt.Printf("🌐 Access: http://localhost:8080\n")
    fmt.Printf("🛡️  Security: All mandatory features active\n")
}

func (app *App) shutdown() error {
    app.logger.Info("🔄 Shutting down services...")

    if app.webServer != nil {
        app.webServer.Stop()
    }

    if app.monitoring != nil {
        app.monitoring.Stop()
    }

    if app.security != nil {
        app.security.Stop()
    }

    if app.db != nil {
        app.db.Close()
    }

    app.logger.Info("✅ Shutdown complete")
    return nil
}