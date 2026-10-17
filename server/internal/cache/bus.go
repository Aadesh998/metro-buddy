package cache

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const channel = "cache:purge"

// Bus fans "purge <name>" out to every API instance so in-process caches stay coherent.
type Bus struct {
	Redis *redis.Client
	Log   *slog.Logger

	mu       sync.RWMutex
	handlers map[string][]func()
}

func (b *Bus) On(name string, fn func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.handlers == nil {
		b.handlers = map[string][]func(){}
	}
	b.handlers[name] = append(b.handlers[name], fn)
}

func (b *Bus) local(name string) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, fn := range b.handlers[name] {
		fn()
	}
}

// Purge clears the named cache here at once and on other instances via Redis.
func (b *Bus) Purge(ctx context.Context, name string) {
	b.local(name)
	if b.Redis != nil {
		if err := b.Redis.Publish(ctx, channel, name).Err(); err != nil && b.Log != nil {
			b.Log.Warn("cache purge publish failed", "cache", name, "err", err.Error())
		}
	}
}

func (b *Bus) Run(ctx context.Context) {
	for ctx.Err() == nil {
		sub := b.Redis.Subscribe(ctx, channel)
		for msg := range sub.Channel() {
			b.local(msg.Payload)
		}
		_ = sub.Close()
		if ctx.Err() == nil {
			time.Sleep(time.Second)
		}
	}
}
