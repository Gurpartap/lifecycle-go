package main

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	_ "github.com/lib/pq"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/Gurpartap/lifecycle-go"
	"github.com/Gurpartap/lifecycle-go/example/providers"
)

type Config struct {
	StopTimeout time.Duration
	DatabaseURL string
	Addr        string
}

type App struct {
	config    Config
	lifecycle *lifecycle.Lifecycle

	logger *logrus.Logger
	db     *sql.DB

	server http.Server
}

func main() {
	app := App{
		config: Config{
			StopTimeout: 60 * time.Second,
			DatabaseURL: "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable",
			Addr:        ":3000",
		},
		lifecycle: &lifecycle.Lifecycle{},
	}

	app.logger = providers.Logger()
	app.db = providers.DB(app.lifecycle, app.logger, "postgres", app.config.DatabaseURL)

	// or
	// // dbProvider implements lifecycle.Service
	// dbProvider := providers.NewDBProvider(app.logger, "postgres", app.config.DatabaseURL)
	// app.lifecycle.Append(dbProvider)

	r := http.NewServeMux()
	r.HandleFunc("/ping", func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusOK)
		rw.Write([]byte("pong"))
	})

	app.server = http.Server{
		Addr:    app.config.Addr,
		Handler: r,
	}

	app.lifecycle.Append(&lifecycle.Hook{
		OnStart: func(_ context.Context) error {
			app.logger.Printf("serving http on %s...", app.server.Addr)
			if err := app.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				return errors.Wrapf(err, "could not begin serving on %s", app.server.Addr)
			}
			return nil
		},
		OnStop: func(ctx context.Context) error {
			app.logger.Println("shutting down server...")
			// http.Server{}.Shutdown(ctx) selects on ctx.Done to kill on stop timeout
			return errors.Wrap(app.server.Shutdown(ctx), "could not shutdown server")
		},
	})

	app.lifecycle.Run(app.logger, app.config.StopTimeout)
}
