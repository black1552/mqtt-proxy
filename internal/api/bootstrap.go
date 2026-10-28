// 引导模式（bootstrap）：服务启动时未检测到配置文件时，仅开放本文件提供的
// 接口，由 Web 页面收集基础运行配置（仅四项，其余使用默认值）、
// 生成配置文件后关闭服务，等待重启进入正常模式。
package api

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"mqtt-proxy/internal/config"
)

// bootstrapStatus 告知前端当前是否处于引导模式，并给出配置文件路径与默认值。
func (s *Server) bootstrapStatus(c *gin.Context) {
	ok(c, gin.H{
		"needs_bootstrap": s.needsBootstrap,
		"config_file":     s.cfg.ConfigFile,
	})
}

// bootstrapReq 是引导页提交的基础运行配置，仅需人工决定的四项，
// 其余配置项使用内置默认值，JWT 密钥由后端自动生成。
type bootstrapReq struct {
	AllowAnonymous  bool   `json:"allow_anonymous"`
	DefaultACLAllow bool   `json:"default_acl_allow"`
	AdminUser       string `json:"admin_user"`
	AdminPass       string `json:"admin_pass"`
}

// validate 校验并归一化提交项，返回可用于写出配置文件的 BootstrapInput。
func (r bootstrapReq) validate() (config.BootstrapInput, string) {
	in := config.BootstrapInput{
		AllowAnonymous:  r.AllowAnonymous,
		DefaultACLAllow: r.DefaultACLAllow,
		AdminUser:       strings.TrimSpace(r.AdminUser),
		AdminPass:       r.AdminPass,
	}
	if in.AdminUser == "" {
		return in, "超级管理员用户名不能为空"
	}
	if len(in.AdminPass) < 6 {
		return in, "超级管理员密码至少 6 位"
	}
	return in, ""
}

// bootstrapSave 接收基础配置，写出配置文件后触发 onSaved（服务随后优雅退出）。
func (s *Server) bootstrapSave(c *gin.Context) {
	var req bootstrapReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "请求体格式不合法")
		return
	}
	in, msg := req.validate()
	if msg != "" {
		fail(c, http.StatusBadRequest, msg)
		return
	}
	path := s.cfg.ConfigFile
	if path == "" {
		path = config.DefaultConfigFile
	}
	if err := config.WriteConfigFile(path, in); err != nil {
		fail(c, http.StatusInternalServerError, "写入配置文件失败: "+err.Error())
		return
	}
	ok(c, gin.H{"config_file": path})
	// 配置已落盘：稍延后通知上层，留出时间把响应送达浏览器，再由上层关闭引导服务。
	if s.onSaved != nil {
		go func() {
			time.Sleep(800 * time.Millisecond)
			s.onSaved()
		}()
	}
}

// RunBootstrap 在引导模式下启动一个最小 HTTP 服务：仅提供内嵌前端、
// 引导 API 与健康检查，不启动数据库与 Broker。
// 返回 saved=true 表示已通过页面保存配置（调用方应重新加载配置并启动完整服务）；
// saved=false 表示收到退出信号且未完成保存。
func RunBootstrap(ctx context.Context, cfg *config.Config, dist fs.FS) (bool, error) {
	savedCh := make(chan struct{})
	var savedOnce sync.Once
	srv := &Server{
		cfg:            cfg,
		needsBootstrap: true,
		onSaved:        func() { savedOnce.Do(func() { close(savedCh) }) },
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(cors())

	r.GET("/api/bootstrap/status", srv.bootstrapStatus)
	r.POST("/api/bootstrap/save", srv.bootstrapSave)
	r.GET("/api/health", func(c *gin.Context) { ok(c, gin.H{"status": "up"}) })
	mountSPA(r, dist)

	httpSrv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	shutdown := func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdownCtx) // 等待在途请求结束并释放监听端口
	}

	select {
	case <-savedCh:
		shutdown()
		return true, nil
	case <-ctx.Done():
		shutdown()
		return false, nil
	case err := <-errCh:
		return false, err
	}
}
