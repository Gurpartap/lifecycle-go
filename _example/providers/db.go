package providers

import (
	"context"
	"database/sql"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/Gurpartap/lifecycle-go"
)

func DB(lc *lifecycle.Lifecycle, logger *logrus.Logger, driverName, databaseURL string) *sql.DB {
	logger.Infoln("initializing db…")

	db, err := sql.Open(driverName, databaseURL)
	if err != nil {
		logger.WithError(err).Fatalln("could not initialize db")
	}

	lc.Append(&lifecycle.Hook{
		OnStop: func(ctx context.Context) error {
			// select on <-ctx.Done() to implement stop timeout
			logger.Infoln("stopping db…")
			return errors.Wrap(db.Close(), "could not stop db")
		},
	})

	return db
}

// alternatively, a service can be any struct implementing lifecycle.Service

func NewDBProvider(logger *logrus.Logger, driverName, databaseURL string) *DBProvider {
	logger.Infoln("initializing db…")

	var err error
	service := &DBProvider{}
	service.DB, err = sql.Open(driverName, databaseURL)
	if err != nil {
		logger.WithError(err).Fatalln("could not initialize db")
	}

	return service
}

type DBProvider struct {
	DB *sql.DB
}

var _ lifecycle.Service = (*DBProvider)(nil)

func (s *DBProvider) Start(_ context.Context) error {
	return nil
}

func (s *DBProvider) Stop(_ context.Context) error {
	return errors.Wrap(s.DB.Close(), "could not stop db")
}
