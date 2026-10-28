// Package model 定义在 repo、service 和 API 各层之间共享的持久化结构体。
package model

import "time"

// 管理员用户的角色取值。
const (
	RoleSuperAdmin = "superadmin"
	RoleAdmin      = "admin"
	RoleReadonly   = "readonly"
	RoleMQTT       = "mqtt" // 普通设备账号，仅用于 Broker 认证
)

// User 是管理员/操作员账号，或一个 MQTT 凭证。
type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	Enabled      bool      `json:"enabled"`
	Remark       string    `json:"remark"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// IsAdmin 表示该账号是否可以登录管理后台。
func (u User) IsAdmin() bool {
	return u.Role == RoleSuperAdmin || u.Role == RoleAdmin || u.Role == RoleReadonly
}

// ACL 访问动作关键字。
const (
	AccessPublish   = "publish"
	AccessSubscribe = "subscribe"
	AccessBoth      = "both"
	EffectAllow     = "allow"
	EffectDeny      = "deny"
)

// ACLRule 对一组客户端的主题访问进行限制或授权。
type ACLRule struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	ClientID   string    `json:"client_id"`
	Username   string    `json:"username"`
	Topic      string    `json:"topic"`
	Access     string    `json:"access"`
	Effect     string    `json:"effect"`
	MaxQoS     int       `json:"max_qos"`
	Priority   int       `json:"priority"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ClientDevice 是一个已登记/已开通的 MQTT 客户端 ID。
type ClientDevice struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Topic 是后台展示的一个受管主题/模式条目。
type Topic struct {
	ID          int64     `json:"id"`
	Topic       string    `json:"topic"`
	Description string    `json:"description"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// StatsSnapshot 是监控趋势历史中的一行快照。
type StatsSnapshot struct {
	ID               int64     `json:"id"`
	TS               time.Time `json:"ts"`
	ClientsConnected int64     `json:"clients_connected"`
	Subscriptions    int64     `json:"subscriptions"`
	MessagesReceived int64     `json:"messages_received"`
	MessagesSent     int64     `json:"messages_sent"`
	Retained         int64     `json:"retained"`
}

// MessageLog 是实时控制台展示的一条近期发布记录。
type MessageLog struct {
	ID        int64     `json:"id"`
	TS        time.Time `json:"ts"`
	ClientID  string    `json:"client_id"`
	Username  string    `json:"username"`
	Topic     string    `json:"topic"`
	QoS       int       `json:"qos"`
	Retained  bool      `json:"retained"`
	Size      int       `json:"size"`
	Direction string    `json:"direction"`
}

// ConnectedClient 是附着在 Broker 上的会话的实时视图。
type ConnectedClient struct {
	ClientID      string    `json:"client_id"`
	Username      string    `json:"username"`
	Remote        string    `json:"remote"`
	Listener      string    `json:"listener"`
	Protocol      string    `json:"protocol"`
	ProtoVersion  int       `json:"proto_version"`
	Keepalive     int       `json:"keepalive"`
	CleanSession  bool      `json:"clean_session"`
	ConnectedAt   time.Time `json:"connected_at"`
	Subscriptions []string  `json:"subscriptions"`
}
