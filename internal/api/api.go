// Package api 对外暴露管理 REST API、JWT 认证中间件以及 React 控制台
// 使用的实时监控 WebSocket。
package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"mqtt-proxy/internal/auth"
	"mqtt-proxy/internal/broker"
	"mqtt-proxy/internal/config"
	"mqtt-proxy/internal/model"
	"mqtt-proxy/internal/repo"
)

// Server 持有所有 handler 共享的依赖。
type Server struct {
	cfg            *config.Config
	store          *repo.Store
	broker         *broker.Service
	issuer         *auth.Issuer
	needsBootstrap bool // 启动时未检测到配置文件，需先完成 Web 引导
	onSaved        func() // 引导模式下配置保存后的回调（触发服务退出）
}

// New 构建一个 API 服务器。needsBootstrap 为 true 表示服务处于引导模式：
// 仅开放 /api/bootstrap 接口，其余 API 在配置文件生成前一律拦截。
func New(cfg *config.Config, store *repo.Store, brk *broker.Service, issuer *auth.Issuer, needsBootstrap bool) *Server {
	return &Server{cfg: cfg, store: store, broker: brk, issuer: issuer, needsBootstrap: needsBootstrap}
}

// ---- 响应封装 ----

type envelope struct {
	Code int    `json:"code"`
	Msg  string `json:"msg,omitempty"`
	Data any    `json:"data,omitempty"`
}

func ok(c *gin.Context, data any) {
	c.JSON(http.StatusOK, envelope{Code: 0, Data: data})
}

func fail(c *gin.Context, status int, msg string) {
	c.JSON(status, envelope{Code: 1, Msg: msg})
}

// ---- 中间件 ----

// bootstrapGuard 在引导模式下拦截除 /api/bootstrap 之外的所有 API，
// 告知前端需先完成基础配置。
func (s *Server) bootstrapGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.needsBootstrap && !strings.HasPrefix(c.Request.URL.Path, "/api/bootstrap") {
			fail(c, http.StatusServiceUnavailable, "服务尚未完成初始化配置，请先在引导页提交基础配置")
			c.Abort()
			return
		}
		c.Next()
	}
}

// authRequired 校验 bearer token 并将载荷存到上下文中。
func (s *Server) authRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.GetHeader("Authorization")
		if raw == "" {
			raw = c.Query("token") // WebSocket 握手时的备选方式
		}
		token := strings.TrimPrefix(raw, "Bearer ")
		if token == "" {
			fail(c, http.StatusUnauthorized, "missing token")
			c.Abort()
			return
		}
		claims, err := s.issuer.Parse(token)
		if err != nil {
			fail(c, http.StatusUnauthorized, "invalid or expired token")
			c.Abort()
			return
		}
		c.Set("uid", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Next()
	}
}

// writeGuard 阻止只读角色执行修改操作。
func (s *Server) writeGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		if role == model.RoleReadonly {
			fail(c, http.StatusForbidden, "read-only account cannot modify resources")
			c.Abort()
			return
		}
		c.Next()
	}
}

func currentRole(c *gin.Context) string {
	if v, ok := c.Get("role"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
