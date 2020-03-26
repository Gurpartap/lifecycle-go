package lifecycle

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
)

var _exit = func() { os.Exit(1) }

type Printer interface {
	Printf(format string, args ...interface{})
}

type Service interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

type Hook struct {
	OnStart func(ctx context.Context) error
	OnStop  func(ctx context.Context) error
}

var _ Service = (*Hook)(nil)

func (s *Hook) Start(ctx context.Context) error {
	return s.OnStart(ctx)
}

func (s *Hook) Stop(ctx context.Context) error {
	return s.OnStop(ctx)
}

type Lifecycle struct {
	hooks []*Hook
}

func (lc *Lifecycle) Append(hook *Hook) {
	lc.hooks = append(lc.hooks, hook)
}

func (lc *Lifecycle) AppendService(service Service) {
	lc.hooks = append(lc.hooks, service.(*Hook))
}

func (lc *Lifecycle) Start(ctx context.Context) error {
	for _, hook := range lc.hooks {
		if hook.OnStart == nil {
			continue
		}
		if err := hook.OnStart(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (lc *Lifecycle) Stop(ctx context.Context) error {
	var errs []error
	for i := len(lc.hooks) - 1; i >= 0; i-- {
		hook := lc.hooks[i]
		if hook.OnStop == nil {
			continue
		}
		if err := hook.OnStop(ctx); err != nil {
			// for best-effort cleanup, keep going after errors
			errs = append(errs, err)
		}
	}
	return multierr.Combine(errs...)
}

func (lc *Lifecycle) Run(logger Printer, stopTimeout time.Duration) {
	lc.RunContext(context.Background(), logger, stopTimeout)
}

func (lc *Lifecycle) RunContext(ctx context.Context, logger Printer, stopTimeout time.Duration) {
	startCtx, cancelStart := context.WithCancel(ctx)
	go func() {
		if err := lc.Start(startCtx); err != nil {
			logger.Printf("ERROR\t\tcould not start lifecycle: %+v\n", err)
			// rollback
			if stopErr := lc.Stop(ctx); stopErr != nil {
				logger.Printf("ERROR\t\tcould not rollback cleanly: %+v", stopErr)
			}
			_exit()
		}
	}()
	_ = lc.waitForSignal(startCtx)
	cancelStart()

	logger.Printf("INFO\t\tattempting clean stop...")
	stopCtx, cancelStop := context.WithTimeout(context.Background(), stopTimeout)
	defer cancelStop()
	if err := lc.Stop(stopCtx); err != nil {
		logger.Printf("ERROR\t\tcould not stop cleanly: %+v\n", err)
		_exit()
	}
	logger.Printf("INFO\t\tstopped cleanly")
}

func (lc *Lifecycle) waitForSignal(ctx context.Context) error {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-c:
		return errors.New("shutdown received")
	case <-ctx.Done():
		return ctx.Err()
	}
}
