// Package app 将配置、存储、Broker 与 HTTP 服务器组装在一起，并管理
// 它们的生命周期。
package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"mqtt-proxy/internal/api"
	"mqtt-proxy/internal/auth"
	"mqtt-proxy/internal/broker"
	"mqtt-proxy/internal/config"
	"mqtt-proxy/internal/db"
	"mqtt-proxy/internal/logging"
	"mqtt-proxy/internal/migrate"
	"mqtt-proxy/internal/model"
	"mqtt-proxy/internal/repo"
)

// App 是顶层的服务容器。
type App struct {
	Cfg       *config.Config
	Log       *slog.Logger
	logWriter io.Closer
	store     *repo.Store
	broker    *broker.Service
	issuer    *auth.Issuer
	distFS    fs.FS
}

// New 打开存储、运行迁移、写入默认管理员种子数据并构建各子系统。
// distFS 是内嵌的前端构建产物（可以为 nil）。
func New(cfg *config.Config, distFS fs.FS) (*App, error) {
	log, logWriter := logging.New(logging.Config{
		File:       cfg.LogFile,
		MaxSizeMB:  cfg.LogMaxSizeMB,
		MaxAgeDays: cfg.LogMaxAgeDays,
		Stdout:     cfg.LogStdout,
		Level:      cfg.LogLevel,
	})

	handle, err := db.Open(cfg.DBPath)
	if err != nil {
		return nil, err
	}
	if _, err := migrate.New(handle).Up(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	store := repo.NewStore(handle)
	if err := seedAdmin(store, cfg); err != nil {
		return nil, err
	}

	issuer := auth.NewIssuer(cfg.JWTSecret, cfg.JWTTTL)
	brk := broker.NewService(cfg, store, log)

	app := &App{Cfg: cfg, Log: log, logWriter: logWriter, store: store, broker: brk, issuer: issuer}
	app.distFS = distFS
	return app, nil
}

// distFS 存于 app 上，以便 Run 构建路由。

// seedAdmin 保证配置的超级管理员账号存在。
func seedAdmin(store *repo.Store, cfg *config.Config) error {
	_, err := store.Users.GetByUsername(cfg.AdminUser)
	if err == nil {
		return nil // 已存在
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	hash, err := auth.HashPassword(cfg.AdminPass)
	if err != nil {
		return err
	}
	return store.Users.Create(&model.User{
		Username:     cfg.AdminUser,
		PasswordHash: hash,
		Role:         model.RoleSuperAdmin,
		Enabled:      true,
		Remark:       "内置超级管理员",
	})
}

// SyncAdminFromConfig 在引导模式生成配置后调用：保证内置超级管理员与配置一致，
// 账号不存在则创建，已存在但密码不同则重置为配置中的密码（避免旧库密码导致无法登录）。
func SyncAdminFromConfig(cfg *config.Config) error {
	handle, err := db.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer handle.Close()
	if _, err := migrate.New(handle).Up(); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	store := repo.NewStore(handle)
	u, err := store.Users.GetByUsername(cfg.AdminUser)
	if errors.Is(err, sql.ErrNoRows) {
		return seedAdmin(store, cfg)
	}
	if err != nil {
		return err
	}
	if auth.CheckPassword(u.PasswordHash, cfg.AdminPass) {
		return nil
	}
	hash, err := auth.HashPassword(cfg.AdminPass)
	if err != nil {
		return err
	}
	return store.Users.UpdatePassword(u.ID, hash)
}

// Run 启动 Broker 与 HTTP 服务器，并阻塞直到 ctx 被取消。
func (a *App) Run(ctx context.Context) error {
	if a.logWriter != nil {
		defer a.logWriter.Close() //nolint
	}
	if err := a.broker.Start(ctx); err != nil {
		return fmt.Errorf("broker start: %w", err)
	}

	// 启动时未检测到配置文件时，服务进入引导模式：其余 API 被拦截，
	// 由 Web 引导页生成配置后重启生效。
	srv := api.New(a.Cfg, a.store, a.broker, a.issuer, !a.Cfg.Exists())
	engine := srv.Router(a.distFS)

	httpSrv := &http.Server{
		Addr:              a.Cfg.HTTPAddr,
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		a.Log.Info("admin http listening", "addr", a.Cfg.HTTPAddr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		a.Log.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdownCtx)
		return a.broker.Stop(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

// Store 暴露仓储层供迁移命令使用。
func (a *App) Store() *repo.Store { return a.store }

// RunMigrate 执行一个独立的迁移命令（up|down|status|force N），针对配置
// 的数据库操作，不启动 Broker。
func RunMigrate(cfg *config.Config, action, arg string) error {
	handle, err := db.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer handle.Close()
	m := migrate.New(handle)

	switch action {
	case "up", "":
		applied, err := m.Up()
		if err != nil {
			return err
		}
		if len(applied) == 0 {
			fmt.Println("database already up to date")
			return nil
		}
		fmt.Printf("applied %d migration(s): %v\n", len(applied), applied)
		store := repo.NewStore(handle)
		return seedAdmin(store, cfg)
	case "down":
		v, err := m.Down()
		if err != nil {
			return err
		}
		fmt.Printf("reverted migration version %d\n", v)
		return nil
	case "force":
		var v int
		if _, err := fmt.Sscanf(arg, "%d", &v); err != nil {
			return fmt.Errorf("force requires a numeric version")
		}
		if err := m.Force(v); err != nil {
			return err
		}
		fmt.Printf("schema version forced to %d\n", v)
		return nil
	case "status":
		cur, err := m.Version()
		if err != nil {
			return err
		}
		all, err := m.All()
		if err != nil {
			return err
		}
		pending, err := m.Pending()
		if err != nil {
			return err
		}
		fmt.Printf("current version: %d\n", cur)
		fmt.Printf("total migrations : %d\n", len(all))
		fmt.Printf("pending          : %d\n", len(pending))
		for _, p := range pending {
			fmt.Printf("  - %04d %s\n", p.Version, p.Name)
		}
		return nil
	default:
		return fmt.Errorf("unknown migrate action %q", action)
	}
}
