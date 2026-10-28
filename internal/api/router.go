package api

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Router 构建完整的 Gin 引擎：/api 下的 JSON API 以及内嵌的 SPA。
func (s *Server) Router(dist fs.FS) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(cors())

	// ---- 引导模式状态（公开）----
	bs := r.Group("/api/bootstrap")
	{
		bs.GET("/status", s.bootstrapStatus)
		// 仅当启动时未检测到配置文件时，才允许通过 Web 页面提交基础配置。
		if s.needsBootstrap {
			bs.POST("/save", s.bootstrapSave)
		}
	}

	// ---- 健康检查（公开）----
	r.GET("/api/health", func(c *gin.Context) { ok(c, gin.H{"status": "up"}) })

	// ---- 登录（公开）----
	r.POST("/api/login", s.bootstrapGuard(), s.login)

	// ---- 需认证 ----
	priv := r.Group("/api", s.bootstrapGuard(), s.authRequired())
	{
		priv.GET("/me", s.me)

		// 仪表盘与监控
		priv.GET("/dashboard", s.dashboard)
		priv.GET("/monitor/info", s.monitorInfo)
		priv.GET("/monitor/history", s.monitorHistory)
		priv.GET("/monitor/messages", s.monitorMessages)
		priv.GET("/monitor/topics", s.monitorTopics)
		priv.GET("/monitor/clients", s.connectedClients)
		priv.POST("/monitor/publish", s.writeGuard(), s.publishMessage)
		priv.GET("/monitor/ws", s.monitorWS)

		// 用户
		priv.GET("/users", s.listUsers)
		priv.POST("/users", s.writeGuard(), s.createUser)
		priv.PUT("/users/:id", s.writeGuard(), s.updateUser)
		priv.DELETE("/users/:id", s.writeGuard(), s.deleteUser)
		priv.POST("/users/:id/password", s.writeGuard(), s.resetPassword)

		// ACL 规则
		priv.GET("/acls", s.listACL)
		priv.POST("/acls", s.writeGuard(), s.createACL)
		priv.PUT("/acls/:id", s.writeGuard(), s.updateACL)
		priv.DELETE("/acls/:id", s.writeGuard(), s.deleteACL)

		// 客户端设备
		priv.GET("/clients", s.listClients)
		priv.POST("/clients", s.writeGuard(), s.createClient)
		priv.PUT("/clients/:id", s.writeGuard(), s.updateClient)
		priv.DELETE("/clients/:id", s.writeGuard(), s.deleteClient)
		priv.POST("/clients/:id/disconnect", s.writeGuard(), s.disconnectClient)

		// 主题
		priv.GET("/topics", s.listTopics)
		priv.POST("/topics", s.writeGuard(), s.createTopic)
		priv.PUT("/topics/:id", s.writeGuard(), s.updateTopic)
		priv.DELETE("/topics/:id", s.writeGuard(), s.deleteTopic)
	}

	// ---- SPA 静态资源 ----
	mountSPA(r, dist)
	return r
}

func cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// mountSPA 服务内嵌的前端，并在未命中时回退到 index.html，
// 使得客户端路由在深链接时也能正常工作。
func mountSPA(r *gin.Engine, dist fs.FS) {
	if dist == nil {
		return
	}
	fileServer := http.FileServer(http.FS(dist))
	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		// 未知的 /api 路径应返回 JSON，而不是 SPA 外壳。
		if strings.HasPrefix(p, "/api/") {
			fail(c, http.StatusNotFound, "not found")
			return
		}
		// 先尝试请求的静态资源。
		name := strings.TrimPrefix(p, "/")
		if name != "" {
			if fi, err := fs.Stat(dist, name); err == nil && !fi.IsDir() {
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
		}
		// 否则服务 SPA 入口页。
		index, err := fs.ReadFile(dist, "index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "frontend not built")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", index)
	})
}
