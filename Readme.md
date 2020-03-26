# 🚴🏻‍♂️ Lifecycle – an application runtime framework for Go apps

[![GoDoc](https://godoc.org/github.com/Gurpartap/lifecycle-go?status.svg)](https://pkg.go.dev/github.com/Gurpartap/lifecycle-go)

This package provides a start, block-until-signal, and stop(timeout) app runtime for multiple services in your Go apps.

Lifecycle provides ability to act on each step of the lifecycle events for every service it runs.

`Append(…)` and `AppendService(…)` calls are additive. Appended services get started in ascending and stopped in descending order.

The last service can either run a blocking task (e.g. a http server), or a non-blocking task (e.g. a bunch of cancellable worker goroutines).

`Run(…)` expects an implementation of `lifecycle.Printer` interface.
It is used to log start and stop errors.
Tested to work with at least `*logrus.Logger`.

A SIGINT, SIGTERM, or cancellation of context provided to `RunContext(…)` triggers a stop.

## Installation

```bash
go get -u https://github.com/Gurpartap/lifecycle-go
```

```go
import "github.com/Gurpartap/lifecycle-go"
```

## Usage

See [_example/cmd/web](https://github.com/Gurpartap/lifecycle-go/blob/master/_example/cmd/web/main.go) and [_example/cmd/cron-jobs](https://github.com/Gurpartap/lifecycle-go/blob/master/_example/cmd/cron-jobs/main.go) for detailed usage examples.

```go
package main

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/Gurpartap/lifecycle-go"
	"github.com/sirupsen/logrus"
)

func main() {
	logger := logrus.StandardLogger()

	lc := &lifecycle.Lifecycle{}

	var db *sql.DB
	// db = providers.DB(lc, logger, driverName, dataSourceName)
	//
	// or
	//
	// dbProvider := providers.NewDBProvider(logger, driverName, dataSourceName)
	// // dbProvider implements lifecycle.Service
	// lc.AppendService(dbProvider)
	//
	// or
	lc.Append(&lifecycle.Hook{
		OnStart: func(_ context.Context) error {
			var err error
			db, err = sql.Open("postgres", "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable")
			if err != nil {
				return err
			}
			return nil
		},
		OnStop: func(ctx context.Context) error {
			// select on <-ctx.Done() to implement stop timeout
			logger.Infoln("stopping db…")
			// http server (below) is shut down before attempting to stop db
			return db.Close()
		},
	})

	r := http.NewServeMux()
	r.HandleFunc("/ping", func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusOK)
		rw.Write([]byte("pong"))
	})

	server := http.Server{
		Addr:    ":3000",
		Handler: r,
	}

	lc.Append(&lifecycle.Hook{
		// only the last service may block inside its OnStart hook
		OnStart: func(ctx context.Context) error {
			// <-ctx.Done emits on stop request
			logger.Infoln("running server…")
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				return err
			}
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Infoln("shutting down server…")
			// <-ctx.Done emits on stop timeout
			return server.Shutdown(ctx)
		},
	})

	// logger implements lifecycle.Printer
	// 15 seconds of stop timeout
	lc.Run(logger, 15*time.Second)
}
```

## Credits

This package takes a [considerable amount of inspiration](https://github.com/uber-go/fx/blob/master/internal/lifecycle/lifecycle.go) from the amazing [uber-go/fx](https://github.com/uber-go/fx) package.
If you're also looking for dependency injection along with an application framework similar to Lifecycle, use fx. 

Lifecycle aims for compile-time type safety while also being open to use of any dependency injection method.

Lifecycle depends on [github.com/pkg/errors](https://github.com/pkg/errors) and [go.uber.org/multierr](https://github.com/uber-go/multierr) for error handling.

## About

    Copyright 2020 Gurpartap Singh

    Licensed under the Apache License, Version 2.0 (the "License");
    you may not use this file except in compliance with the License.
    You may obtain a copy of the License at

        http://www.apache.org/licenses/LICENSE-2.0

    Unless required by applicable law or agreed to in writing, software
    distributed under the License is distributed on an "AS IS" BASIS,
    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    See the License for the specific language governing permissions and
    limitations under the License.
