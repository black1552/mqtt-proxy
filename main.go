// Command mqtt-proxy 是一个内嵌式多版本 MQTT Broker，自带管理后台。
// React/AntD 前端会编译到 admin/dist 并在此内嵌，因此单个二进制即可同时
// 提供 API 与管理界面。
package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"syscall"

	"mqtt-proxy/internal/api"
	"mqtt-proxy/internal/app"
	"mqtt-proxy/internal/config"
)

// 发布构建拆为两步，由 go generate 驱动（IDE 中可直接点击指令执行）：
// 第一步：编译内嵌的管理后台前端（admin/ → admin/dist）。
//go:generate go run gen.go frontend
// 第二步：go build 交叉编译 Linux 发布二进制，产物在 release/ 目录。
//go:generate go run gen.go linux
// 可选：如需同时产出 Windows 发布包，取消下行注释（或单独运行 go run gen.go windows）。
//go:generate go run gen.go windows

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	args := os.Args[1:]

	// 允许用 -config 指定配置文件路径（写入环境变量供 config.Load 读取）。
	for i, a := range args {
		if (a == "-config" || a == "--config") && i+1 < len(args) {
			os.Setenv("CONFIG_FILE", args[i+1])
		}
	}

	// 生成默认配置模板子命令：`mqtt-proxy gen-config [路径]`。
	if len(args) > 0 && args[0] == "gen-config" {
		path := config.DefaultConfigFile
		if len(args) > 1 {
			path = args[1]
		}
		if err := config.WriteTemplate(path); err != nil {
			return err
		}
		fmt.Printf("已生成默认配置模板: %s\n", path)
		return nil
	}

	cfg := config.Load()

	// 独立的迁移工具：`mqtt-proxy migrate up|down|status|force N`。
	if len(args) > 0 && args[0] == "migrate" {
		action, arg := "up", ""
		if len(args) > 1 {
			action = args[1]
		}
		if len(args) > 2 {
			arg = args[2]
		}
		return app.RunMigrate(cfg, action, arg)
	}

	dist, err := fs.Sub(distFS, "admin/dist")
	if err != nil {
		return fmt.Errorf("embedded frontend: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 启动时优先检测配置文件：不存在则进入引导模式，由 Web 页面配置基础运行配置；
	// 保存后服务自动重新加载新配置并启动，前端随后跳转到登录页面。
	if !cfg.Exists() {
		fmt.Printf("未检测到配置文件 %q，服务以引导模式启动。\n", cfg.ConfigFile)
		fmt.Printf("请打开 http://localhost%s 完成基础配置，保存后自动加载新配置并启动服务。\n", cfg.HTTPAddr)
		saved, err := api.RunBootstrap(ctx, cfg, dist)
		if err != nil {
			return err
		}
		if !saved {
			return nil // 未完成配置即收到退出信号
		}
		cfg = config.Load() // 此时配置文件已写入，重新读取生效
		if err := app.SyncAdminFromConfig(cfg); err != nil {
			return err
		}
		fmt.Printf("已生成并加载配置文件 %q，服务继续使用新配置启动。\n", cfg.ConfigFile)
	}

	instance, err := app.New(cfg, dist)
	if err != nil {
		return err
	}

	fmt.Printf("mqtt-proxy admin console: http://localhost%s  (默认账号 %s / %s)\n",
		cfg.HTTPAddr, cfg.AdminUser, cfg.AdminPass)
	return instance.Run(ctx)
}
