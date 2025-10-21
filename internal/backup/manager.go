package backup

import (
    "github.com/casapps/casspaces/internal/database"
    "github.com/sirupsen/logrus"
)

type Manager struct {
    db     database.Database
    config interface{}
    paths  interface{}
    logger *logrus.Logger
}

func New(db database.Database, config, paths interface{}, logger *logrus.Logger) (*Manager, error) {
    return &Manager{
        db:     db,
        config: config,
        paths:  paths,
        logger: logger,
    }, nil
}

func (m *Manager) Stop() {
    m.logger.Info("✅ Backup manager stopped")
}