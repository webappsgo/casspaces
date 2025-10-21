package workspace

import (
    "github.com/casapps/casspaces/internal/database"
    "github.com/sirupsen/logrus"
)

type Manager struct {
    db     database.Database
    config interface{}
    logger *logrus.Logger
}

func New(db database.Database, config interface{}, logger *logrus.Logger) (*Manager, error) {
    logger.Info("✅ Workspace manager initialized")
    return &Manager{
        db:     db,
        config: config,
        logger: logger,
    }, nil
}