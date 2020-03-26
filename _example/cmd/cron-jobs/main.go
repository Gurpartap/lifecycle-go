package main

import (
	"context"
	"database/sql"
	"sync"
	"time"

	_ "github.com/lib/pq"
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

	var stoppedSignals []<-chan struct{}

	app.lifecycle.Append(&lifecycle.Hook{
		OnStart: func(ctx context.Context) error {
			app.logger.Infoln("starting cron jobs…")

			// the jobs started can select on ctx.Done to determine when to stop
			stoppedSignals = append(stoppedSignals, []<-chan struct{}{
				_every(ctx, 1*time.Minute, sendEmails(app.logger, app.db)),
				_every(ctx, 5*time.Minute, doHousekeepingTasks(app.logger, app.db)),
			}...)

			app.logger.Printf("started cron jobs")

			return nil
		},
		OnStop: func(ctx context.Context) error {
			app.logger.Println("stopping cron jobs…")

			// select on ctx.Done to kill on stop timeout
			select {
			case <-_whenAllStopped(stoppedSignals):
				break
			case <-ctx.Done():
				app.logger.Warnln("timed out trying to stop cron jobs")
			}

			return nil
		},
	})

	app.lifecycle.Run(app.logger, app.config.StopTimeout)
}

func sendEmails(logger *logrus.Logger, db *sql.DB) func(ctx context.Context) {
	return func(ctx context.Context) {
		logger.Infoln("sending emails queued in db…")

		// did we receive a stop command?
		select {
		case <-ctx.Done():
			// time to stop
			return
		default:
			// keep going
		}

		// do work
		time.Sleep(2 * time.Second)

		logger.Infoln("emails sent")
	}
}

func doHousekeepingTasks(logger *logrus.Logger, db *sql.DB) func(ctx context.Context) {
	return func(ctx context.Context) {
		logger.Infoln("deleting stale data from db…")

		// did we receive a stop command?
		select {
		case <-ctx.Done():
			// time to stop
			return
		default:
			// keep going
		}

		// do work
		time.Sleep(5 * time.Second)

		logger.Infoln("clean up done")
	}
}

func _every(ctx context.Context, interval time.Duration, fn func(ctx context.Context)) <-chan struct{} {
	stopped := make(chan struct{})
	firstRun := make(chan struct{})
	ticker := time.NewTicker(interval)

	go func() {
		for {
			select {
			case <-firstRun:
				fn(ctx)
			case <-ticker.C:
				fn(ctx)
			case <-ctx.Done():
				ticker.Stop()
				stopped <- struct{}{}
				return
			}
		}
	}()

	firstRun <- struct{}{}

	return stopped
}

func _whenAllStopped(stoppedSignals []<-chan struct{}) <-chan struct{} {
	stopped := make(chan struct{}, 1)
	wg := sync.WaitGroup{}

	for i := range stoppedSignals {
		wg.Add(1)
		go func(ch <-chan struct{}) {
			<-ch
			wg.Done()
		}(stoppedSignals[i])
	}

	go func() {
		wg.Wait()
		stopped <- struct{}{}
	}()

	return stopped
}
