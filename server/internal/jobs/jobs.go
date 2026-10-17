package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

type Runner struct {
	Redis *redis.Client
	Log   *slog.Logger
}

// Every runs fn on one instance per interval: a Redis lock held for the interval
// stops other API instances from running the same job concurrently.
func (r *Runner) Every(ctx context.Context, name string, interval time.Duration, fn func(context.Context) error) {
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			r.runOnce(ctx, name, interval, fn)
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()
}

func (r *Runner) runOnce(ctx context.Context, name string, interval time.Duration, fn func(context.Context) error) {
	ok, err := r.Redis.SetNX(ctx, "job:"+name, "1", max(interval-time.Second, interval/2)).Result()
	if err != nil || !ok {
		return
	}
	defer func() {
		if p := recover(); p != nil {
			r.Log.Error("job panicked", "job", name)
		}
	}()
	start := time.Now()
	if err := fn(ctx); err != nil {
		r.Log.Error("job failed", "job", name, "err", err.Error())
		return
	}
	level := slog.LevelInfo
	if interval < time.Minute {
		level = slog.LevelDebug
	}
	r.Log.Log(ctx, level, "job done", "job", name, "dur_ms", time.Since(start).Milliseconds())
}
