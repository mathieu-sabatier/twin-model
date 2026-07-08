package app

import (
	"context"
	"time"

	"go.uber.org/fx"

	"github.com/mathieu-sabatier/twin-model/internal/core"
)

// CoreModule provides the shared domain: Config, the draft Store (with its TTL
// sweeper bound to the fx lifecycle), the GitHost, and the Service. Both the
// serve and mcp apps compose this module.
func CoreModule() fx.Option {
	return fx.Module("core",
		fx.Provide(
			core.ConfigFromEnv,
			newStore,
			core.NewGitHost,
			core.New, // (host GitHost, store *Store) -> *Service
		),
	)
}

// newStore builds the draft Store and starts/stops its sweeper via the lifecycle.
func newStore(lc fx.Lifecycle, cfg core.Config) *core.Store {
	store := core.NewStore(cfg.TTL)
	ctx, cancel := context.WithCancel(context.Background())
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error { go store.StartSweeper(ctx, cfg.TTL/4+time.Minute); return nil },
		OnStop:  func(context.Context) error { cancel(); return nil },
	})
	return store
}
