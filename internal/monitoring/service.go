package monitoring

import (
    "context"
    "github.com/casapps/casspaces/internal/database"
    "github.com/sirupsen/logrus"
)

type Service struct {
    db     database.Database
    config interface{}
    logger *logrus.Logger
}

func New(db database.Database, config interface{}, logger *logrus.Logger) (*Service, error) {
    return &Service{
        db:     db,
        config: config,
        logger: logger,
    }, nil
}

func (s *Service) Start(ctx context.Context) error {
    s.logger.Info("✅ Monitoring service started")
    return nil
}

func (s *Service) Stop() {
    s.logger.Info("✅ Monitoring service stopped")
}