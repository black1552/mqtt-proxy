package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"mqtt-proxy/internal/auth"
	"mqtt-proxy/internal/model"
)

func idParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		fail(c, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}

// ------------------------------------------------------------------- auth ---

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	user, err := s.store.Users.GetByUsername(strings.TrimSpace(req.Username))
	if err != nil || !user.IsAdmin() || !user.Enabled || !auth.CheckPassword(user.PasswordHash, req.Password) {
		fail(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	token, err := s.issuer.Sign(user.ID, user.Username, user.Role)
	if err != nil {
		fail(c, http.StatusInternalServerError, "签发令牌失败")
		return
	}
	ok(c, gin.H{"token": token, "user": user})
}

func (s *Server) me(c *gin.Context) {
	user, err := s.store.Users.Get(int64(c.GetInt("uid")))
	if err != nil {
		fail(c, http.StatusNotFound, "user not found")
		return
	}
	ok(c, user)
}

// -------------------------------------------------------------- dashboard ---

func (s *Server) dashboard(c *gin.Context) {
	users, _ := s.store.Users.Count()
	acls, _ := s.store.ACL.Count()
	clients, _ := s.store.Clients.Count()
	topics, _ := s.store.Topics.Count()
	connected := len(s.broker.ConnectedClients())
	ok(c, gin.H{
		"users":             users,
		"acl_rules":         acls,
		"registered":        clients,
		"managed_topics":    topics,
		"connected":         connected,
		"broker":            s.broker.Info(),
	})
}

// ------------------------------------------------------------- monitoring ---

func (s *Server) monitorInfo(c *gin.Context) { ok(c, s.broker.Info()) }

func (s *Server) monitorHistory(c *gin.Context) {
	n := parseIntDefault(c.Query("limit"), 120)
	items, err := s.store.Stats.RecentSnapshots(n)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, items)
}

func (s *Server) monitorMessages(c *gin.Context) {
	n := parseIntDefault(c.Query("limit"), 50)
	ok(c, s.broker.RecentMessages(n))
}

func (s *Server) monitorTopics(c *gin.Context) { ok(c, s.broker.LiveTopics()) }

func (s *Server) connectedClients(c *gin.Context) { ok(c, s.broker.ConnectedClients()) }

type publishReq struct {
	Topic   string `json:"topic"`
	Payload string `json:"payload"`
	QoS     int    `json:"qos"`
	Retain  bool   `json:"retain"`
}

func (s *Server) publishMessage(c *gin.Context) {
	var req publishReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Topic == "" {
		fail(c, http.StatusBadRequest, "topic required")
		return
	}
	if err := s.broker.Publish(req.Topic, []byte(req.Payload), byte(req.QoS), req.Retain); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, gin.H{"published": true})
}

// ------------------------------------------------------------------ users ---

func (s *Server) listUsers(c *gin.Context) {
	includeMQTT := c.Query("include_mqtt") == "1"
	items, err := s.store.Users.List(includeMQTT, c.Query("q"))
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, items)
}

type userReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
	Enabled  *bool  `json:"enabled"`
	Remark   string `json:"remark"`
}

func validRole(r string) bool {
	switch r {
	case model.RoleSuperAdmin, model.RoleAdmin, model.RoleReadonly, model.RoleMQTT:
		return true
	}
	return false
}

func (s *Server) createUser(c *gin.Context) {
	var req userReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Username == "" || req.Password == "" {
		fail(c, http.StatusBadRequest, "username and password required")
		return
	}
	if req.Role == "" {
		req.Role = model.RoleMQTT
	}
	if !validRole(req.Role) {
		fail(c, http.StatusBadRequest, "invalid role")
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	u := &model.User{Username: strings.TrimSpace(req.Username), PasswordHash: hash, Role: req.Role, Enabled: enabled, Remark: req.Remark}
	if err := s.store.Users.Create(u); err != nil {
		if isUniqueErr(err) {
			fail(c, http.StatusConflict, "用户名已存在")
			return
		}
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	s.broker.ReloadRules()
	ok(c, u)
}

func (s *Server) updateUser(c *gin.Context) {
	id, good := idParam(c)
	if !good {
		return
	}
	u, err := s.store.Users.Get(id)
	if err != nil {
		fail(c, http.StatusNotFound, "user not found")
		return
	}
	var req userReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Username != "" {
		u.Username = strings.TrimSpace(req.Username)
	}
	if req.Role != "" {
		if !validRole(req.Role) {
			fail(c, http.StatusBadRequest, "invalid role")
			return
		}
		u.Role = req.Role
	}
	if req.Enabled != nil {
		u.Enabled = *req.Enabled
	}
	u.Remark = req.Remark
	if err := s.store.Users.Update(&u); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, u)
}

func (s *Server) deleteUser(c *gin.Context) {
	id, good := idParam(c)
	if !good {
		return
	}
	target, err := s.store.Users.Get(id)
	if err != nil {
		fail(c, http.StatusNotFound, "user not found")
		return
	}
	if target.Role == model.RoleSuperAdmin {
		fail(c, http.StatusForbidden, "不能删除超级管理员")
		return
	}
	if err := s.store.Users.Delete(id); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, gin.H{"deleted": true})
}

func (s *Server) resetPassword(c *gin.Context) {
	id, good := idParam(c)
	if !good {
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Password == "" {
		fail(c, http.StatusBadRequest, "password required")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.store.Users.UpdatePassword(id, hash); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, gin.H{"updated": true})
}

// ------------------------------------------------------------------- acl ----

func (s *Server) listACL(c *gin.Context) {
	items, err := s.store.ACL.List(c.Query("q"))
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, items)
}

type aclReq struct {
	Name     string `json:"name"`
	ClientID string `json:"client_id"`
	Username string `json:"username"`
	Topic    string `json:"topic"`
	Access   string `json:"access"`
	Effect   string `json:"effect"`
	MaxQoS   int    `json:"max_qos"`
	Priority int    `json:"priority"`
	Enabled  *bool  `json:"enabled"`
}

func (a aclReq) toRule(id int64) model.ACLRule {
	enabled := true
	if a.Enabled != nil {
		enabled = *a.Enabled
	}
	return model.ACLRule{
		ID: id, Name: a.Name, ClientID: a.ClientID, Username: a.Username, Topic: a.Topic,
		Access: a.Access, Effect: a.Effect, MaxQoS: a.MaxQoS, Priority: a.Priority, Enabled: enabled,
	}
}

func normalizeACL(a *aclReq) {
	if a.Access == "" {
		a.Access = model.AccessBoth
	}
	if a.Effect == "" {
		a.Effect = model.EffectAllow
	}
	if a.Priority == 0 {
		a.Priority = 100
	}
}

func validACL(a aclReq) bool {
	if a.Name == "" || a.Topic == "" {
		return false
	}
	switch a.Access {
	case model.AccessPublish, model.AccessSubscribe, model.AccessBoth:
	default:
		return false
	}
	switch a.Effect {
	case model.EffectAllow, model.EffectDeny:
	default:
		return false
	}
	return true
}

func (s *Server) createACL(c *gin.Context) {
	var req aclReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	normalizeACL(&req)
	if !validACL(req) {
		fail(c, http.StatusBadRequest, "name/topic required, access & effect must be valid")
		return
	}
	rule := req.toRule(0)
	if err := s.store.ACL.Create(&rule); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	s.broker.ReloadRules()
	ok(c, rule)
}

func (s *Server) updateACL(c *gin.Context) {
	id, good := idParam(c)
	if !good {
		return
	}
	if _, err := s.store.ACL.Get(id); err != nil {
		fail(c, http.StatusNotFound, "rule not found")
		return
	}
	var req aclReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	normalizeACL(&req)
	if !validACL(req) {
		fail(c, http.StatusBadRequest, "name/topic required, access & effect must be valid")
		return
	}
	rule := req.toRule(id)
	if err := s.store.ACL.Update(&rule); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	s.broker.ReloadRules()
	ok(c, rule)
}

func (s *Server) deleteACL(c *gin.Context) {
	id, good := idParam(c)
	if !good {
		return
	}
	if err := s.store.ACL.Delete(id); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	s.broker.ReloadRules()
	ok(c, gin.H{"deleted": true})
}

// ---------------------------------------------------------------- clients ---

func (s *Server) listClients(c *gin.Context) {
	items, err := s.store.Clients.List(c.Query("q"))
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, items)
}

type clientReq struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     *bool  `json:"enabled"`
}

func (s *Server) createClient(c *gin.Context) {
	var req clientReq
	if err := c.ShouldBindJSON(&req); err != nil || req.ID == "" {
		fail(c, http.StatusBadRequest, "client id required")
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	dev := &model.ClientDevice{ID: req.ID, Name: req.Name, Description: req.Description, Enabled: enabled}
	if _, err := s.store.Clients.Get(req.ID); err == nil {
		fail(c, http.StatusConflict, "客户端已存在")
		return
	}
	if err := s.store.Clients.Upsert(dev); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, dev)
}

func (s *Server) updateClient(c *gin.Context) {
	id := c.Param("id")
	dev, err := s.store.Clients.Get(id)
	if err != nil {
		fail(c, http.StatusNotFound, "client not found")
		return
	}
	var req clientReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Name != "" {
		dev.Name = req.Name
	}
	dev.Description = req.Description
	if req.Enabled != nil {
		dev.Enabled = *req.Enabled
	}
	if err := s.store.Clients.Update(&dev); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, dev)
}

func (s *Server) deleteClient(c *gin.Context) {
	id := c.Param("id")
	if err := s.store.Clients.Delete(id); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, gin.H{"deleted": true})
}

func (s *Server) disconnectClient(c *gin.Context) {
	id := c.Param("id")
	if !s.broker.Disconnect(id) {
		fail(c, http.StatusNotFound, "client not connected")
		return
	}
	ok(c, gin.H{"disconnected": true})
}

// ----------------------------------------------------------------- topics ---

func (s *Server) listTopics(c *gin.Context) {
	items, err := s.store.Topics.List(c.Query("q"))
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, items)
}

type topicReq struct {
	Topic       string `json:"topic"`
	Description string `json:"description"`
	Enabled     *bool  `json:"enabled"`
}

func (s *Server) createTopic(c *gin.Context) {
	var req topicReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Topic == "" {
		fail(c, http.StatusBadRequest, "topic required")
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	t := &model.Topic{Topic: req.Topic, Description: req.Description, Enabled: enabled}
	if err := s.store.Topics.Create(t); err != nil {
		if isUniqueErr(err) {
			fail(c, http.StatusConflict, "主题已存在")
			return
		}
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, t)
}

func (s *Server) updateTopic(c *gin.Context) {
	id, good := idParam(c)
	if !good {
		return
	}
	t, err := s.store.Topics.Get(id)
	if err != nil {
		fail(c, http.StatusNotFound, "topic not found")
		return
	}
	var req topicReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Topic != "" {
		t.Topic = req.Topic
	}
	t.Description = req.Description
	if req.Enabled != nil {
		t.Enabled = *req.Enabled
	}
	if err := s.store.Topics.Update(&t); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, t)
}

func (s *Server) deleteTopic(c *gin.Context) {
	id, good := idParam(c)
	if !good {
		return
	}
	if err := s.store.Topics.Delete(id); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, gin.H{"deleted": true})
}

// ------------------------------------------------------------------ helpers -

func parseIntDefault(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil && n > 0 {
		return n
	}
	return def
}

func isUniqueErr(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unique")
}
