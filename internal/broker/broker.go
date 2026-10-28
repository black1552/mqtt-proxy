// Package broker 内嵌一个 mochi-mqtt 服务器并与管理面打通。它接受
// MQTT v3.1（协议 3）、v3.1.1（协议 4）和 v5（协议 5）客户端，
// 执行存于 SQLite 的用户认证与 ACL 规则，并把实时的会话/消息状态
// 暴露给监控 API。
package broker

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"
	"github.com/mochi-mqtt/server/v2/system"

	"mqtt-proxy/internal/auth"
	"mqtt-proxy/internal/config"
	"mqtt-proxy/internal/match"
	"mqtt-proxy/internal/model"
	"mqtt-proxy/internal/repo"
)

// 推送给监控 WebSocket 中心的事件类型。
const (
	EventClientConnected    = "client_connected"
	EventClientDisconnected = "client_disconnected"
	EventMessage            = "message"
)

// Event 是一条实时的 Broker 通知。
type Event struct {
	Type      string            `json:"type"`
	Timestamp time.Time         `json:"ts"`
	Client    *model.ConnectedClient `json:"client,omitempty"`
	Message   *model.MessageLog `json:"message,omitempty"`
	Detail    string            `json:"detail,omitempty"`
}

// Service 拥有内嵌的 Broker 及其运行时状态。
type Service struct {
	cfg   *config.Config
	store *repo.Store
	log   *slog.Logger

	server *mqtt.Server

	mu      sync.RWMutex
	clients map[string]*mqtt.Client          // 活指针，用于强制断开
	live    map[string]model.ConnectedClient // 供 API 使用的快照视图

	topics  *topicTable // 分片主题计数器，发布热路径无全局锁
	msgRing *msgRing    // 近期消息环形缓冲，供监控读取（不落库）

	info    atomic.Pointer[system.Info]
	rules   atomic.Value // []model.ACLRule
	rulesAt atomic.Int64

	hubMu    sync.Mutex
	hubs     map[chan Event]struct{}
	hubCount atomic.Int32 // 在线监控订阅者数量，为 0 时跳过事件分发
	eventQ   chan Event   // 单一事件队列，避免每条消息 spawn goroutine
}

// NewService 构造 Broker 服务。
func NewService(cfg *config.Config, store *repo.Store, log *slog.Logger) *Service {
	s := &Service{
		cfg:     cfg,
		store:   store,
		log:     log,
		clients: map[string]*mqtt.Client{},
		live:    map[string]model.ConnectedClient{},
		topics:  newTopicTable(),
		msgRing: newMsgRing(liveMessageRingSize),
		hubs:    map[chan Event]struct{}{},
		eventQ:  make(chan Event, 1024),
	}
	s.rules.Store([]model.ACLRule{})
	return s
}

// Start 构建 mochi-mqtt 服务器，挂载监听器和 ACL/认证钩子。
func (s *Service) Start(ctx context.Context) error {
	caps := mqtt.NewDefaultServerCapabilities()
	// 接受我们所公告的最旧协议，使 v3.1 客户端可以连接。
	if len(s.cfg.AcceptProtocolVersions) > 0 {
		var minV byte = 5
		for _, v := range s.cfg.AcceptProtocolVersions {
			if v < minV {
				minV = v
			}
		}
		caps.MinimumProtocolVersion = minV
	}

	s.server = mqtt.New(&mqtt.Options{
		InlineClient: true,
		Capabilities: caps,
		Logger:       s.log,
	})

	if err := s.server.AddHook(new(aclHook{svc: s}), nil); err != nil {
		return err
	}

	tcp := listeners.NewTCP(listeners.Config{ID: "tcp", Address: s.cfg.MQTTTCPAddr})
	if err := s.server.AddListener(tcp); err != nil {
		return err
	}

	ws := listeners.NewWebsocket(listeners.Config{ID: "ws", Address: s.cfg.MQTTWSAddr})
	if err := s.server.AddListener(ws); err != nil {
		return err
	}

	if err := s.server.Serve(); err != nil {
		return err
	}

	go s.eventPump(ctx)
	go s.rulesRefresher(ctx)
	go s.snapshotter(ctx)
	s.log.Info("broker started", "tcp", s.cfg.MQTTTCPAddr, "ws", s.cfg.MQTTWSAddr)
	return nil
}

// snapshotter 定期持久化 Broker 计数器，供趋势图使用。
func (s *Service) snapshotter(ctx context.Context) {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			info := s.info.Load()
			if info == nil {
				continue
			}
			snap := model.StatsSnapshot{
				TS:               time.Now(),
				ClientsConnected: int64(info.ClientsConnected),
				Subscriptions:    int64(info.Subscriptions),
				MessagesReceived: int64(info.MessagesReceived),
				MessagesSent:     int64(info.MessagesSent),
				Retained:         int64(info.Retained),
			}
			if err := s.store.Stats.AddSnapshot(snap); err != nil {
				s.log.Warn("snapshot write failed", "err", err)
			}
			_ = s.store.Stats.TrimSnapshots(720)
		}
	}
}

// Stop 关闭 Broker。
func (s *Service) Stop(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Close()
}

// Publish 通过内联客户端发送一条消息（绕过 ACL）。
func (s *Service) Publish(topic string, payload []byte, qos byte, retain bool) error {
	return s.server.Publish(topic, payload, retain, qos)
}

// ----------------------------------------------------------------- clients --

// ConnectedClients 返回实时会话列表。
func (s *Service) ConnectedClients() []model.ConnectedClient {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.ConnectedClient, 0, len(s.live))
	for _, c := range s.live {
		out = append(out, c)
	}
	return out
}

// Disconnect 根据 id 强制关闭一个客户端的 socket。
func (s *Service) Disconnect(clientID string) bool {
	s.mu.RLock()
	cl := s.clients[clientID]
	s.mu.RUnlock()
	if cl == nil {
		return false
	}
	if cl.Net.Conn != nil {
		_ = cl.Net.Conn.Close()
	}
	return true
}

func protoName(v byte) string {
	switch v {
	case 3:
		return "3.1"
	case 4:
		return "3.1.1"
	case 5:
		return "5.0"
	default:
		return "unknown"
	}
}

// --------------------------------------------------------------- snapshot --

// Info 以通用 map 的形式返回最新的 $SYS 计数器。
func (s *Service) Info() map[string]any {
	info := s.info.Load()
	if info == nil {
		return map[string]any{}
	}
	b, err := json.Marshal(info)
	if err != nil {
		return map[string]any{}
	}
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}

// LiveTopics 返回启动以来观测到的主题。
func (s *Service) LiveTopics() map[string]int {
	return s.topics.snapshot()
}

// ------------------------------------------------------------ subscriptions -

// Subscribe 注册一个监控中心通道。
func (s *Service) Subscribe() chan Event {
	ch := make(chan Event, 64)
	s.hubMu.Lock()
	s.hubs[ch] = struct{}{}
	s.hubCount.Add(1)
	s.hubMu.Unlock()
	return ch
}

// Unsubscribe 移除并关闭一个中心通道。
func (s *Service) Unsubscribe(ch chan Event) {
	s.hubMu.Lock()
	if _, ok := s.hubs[ch]; ok {
		delete(s.hubs, ch)
		s.hubCount.Add(-1)
		s.hubMu.Unlock()
		close(ch)
		return
	}
	s.hubMu.Unlock()
}

func (s *Service) emit(ev Event) {
	s.hubMu.Lock()
	defer s.hubMu.Unlock()
	for ch := range s.hubs {
		select {
		case ch <- ev:
		default:
			// 向慢消费者丢弃事件；监控绝不能阻塞 Broker
		}
	}
}

// -------------------------------------------------------- acl rule caching --

// ReloadRules 强制 ACL 缓存在下次检查时刷新。
func (s *Service) ReloadRules() { s.rulesAt.Store(0) }

func (s *Service) currentRules() []model.ACLRule {
	if time.Now().Unix()-s.rulesAt.Load() > 2 {
		rules, err := s.store.ACL.EnabledRules()
		if err == nil {
			s.rules.Store(rules)
			s.rulesAt.Store(time.Now().Unix())
		}
	}
	return s.rules.Load().([]model.ACLRule)
}

func (s *Service) rulesRefresher(ctx context.Context) {
	t := time.NewTicker(3 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.currentRules()
		}
	}
}

// --------------------------------------------------------- message feed ----

// eventPump 从单一事件队列取出监控事件并广播，避免每条消息创建 goroutine。
func (s *Service) eventPump(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-s.eventQ:
			s.emit(ev)
		}
	}
}

// publishEvent 将事件投递到监控队列；无人订阅时直接丢弃，队列满时也不阻塞 Broker。
func (s *Service) publishEvent(ev Event) {
	if s.hubCount.Load() == 0 {
		return
	}
	select {
	case s.eventQ <- ev:
	default:
	}
}

// RecentMessages 返回最近 n 条消息（从新到旧），数据来自内存环形缓冲。
func (s *Service) RecentMessages(n int) []model.MessageLog {
	return s.msgRing.recent(n)
}

// ------------------------------------------------------------------- hook --

// aclHook 为 Broker 实现认证、ACL 和实时状态跟踪。
type aclHook struct {
	mqtt.HookBase
	svc *Service
}

func (h *aclHook) ID() string { return "acl-auth-hook" }

func (h *aclHook) Provides(b byte) bool {
	return b == mqtt.OnConnectAuthenticate ||
		b == mqtt.OnACLCheck ||
		b == mqtt.OnSessionEstablished ||
		b == mqtt.OnDisconnect ||
		b == mqtt.OnPublish ||
		b == mqtt.OnSysInfoTick
}

func (h *aclHook) OnSysInfoTick(info *system.Info) {
	clone := *info
	h.svc.info.Store(&clone)
}

func (h *aclHook) OnConnectAuthenticate(cl *mqtt.Client, pk packets.Packet) bool {
	svc := h.svc
	username := string(pk.Connect.Username)
	password := string(pk.Connect.Password)

	// 已登记设备拦截：已知但被禁用的客户端 id 始终拒绝。
	if dev, err := svc.store.Clients.Get(cl.ID); err == nil && !dev.Enabled {
		return false
	}

	if username == "" {
		return svc.cfg.AllowAnonymous
	}

	user, err := svc.store.Users.GetByUsername(username)
	if err != nil {
		return false
	}
	if !user.Enabled {
		return false
	}
	return auth.CheckPassword(user.PasswordHash, password)
}

func (h *aclHook) OnACLCheck(cl *mqtt.Client, topic string, write bool) bool {
	svc := h.svc
	username := string(cl.Properties.Username)

	// 超级管理员账号绕过所有 ACL 检查。
	if username != "" {
		if u, err := svc.store.Users.GetByUsername(username); err == nil && u.Role == model.RoleSuperAdmin {
			return true
		}
	}

	rules := svc.currentRules()
	access := model.AccessSubscribe
	if write {
		access = model.AccessPublish
	}

	for _, r := range rules { // 已按优先级排序
		if r.Access != access && r.Access != model.AccessBoth {
			continue
		}
		if !match.Glob(r.ClientID, cl.ID) || !match.Glob(r.Username, username) {
			continue
		}
		if !match.Topic(r.Topic, topic) {
			continue
		}
		return r.Effect == model.EffectAllow
	}
	// 无规则命中 -> 回退到配置的默认策略。
	return svc.cfg.DefaultACLAllow
}

func (h *aclHook) OnSessionEstablished(cl *mqtt.Client, pk packets.Packet) {
	svc := h.svc
	cc := model.ConnectedClient{
		ClientID:     cl.ID,
		Username:     string(cl.Properties.Username),
		Remote:       cl.Net.Remote,
		Listener:     cl.Net.Listener,
		Protocol:     protoName(cl.Properties.ProtocolVersion),
		ProtoVersion: int(cl.Properties.ProtocolVersion),
		CleanSession: cl.Properties.Clean,
		ConnectedAt:  time.Now(),
	}
	svc.mu.Lock()
	svc.clients[cl.ID] = cl
	svc.live[cl.ID] = cc
	svc.mu.Unlock()
	snapshot := cc
	svc.publishEvent(Event{Type: EventClientConnected, Timestamp: cc.ConnectedAt, Client: &snapshot})
}

func (h *aclHook) OnDisconnect(cl *mqtt.Client, err error, expire bool) {
	svc := h.svc
	svc.mu.Lock()
	removed := false
	if _, ok := svc.live[cl.ID]; ok {
		delete(svc.live, cl.ID)
		delete(svc.clients, cl.ID)
		removed = true
	}
	svc.mu.Unlock()
	if removed {
		id := cl.ID
		svc.publishEvent(Event{Type: EventClientDisconnected, Timestamp: time.Now(), Detail: id})
	}
}

func (h *aclHook) OnPublish(cl *mqtt.Client, pk packets.Packet) (packets.Packet, error) {
	svc := h.svc
	// 主题计数走分片表，热路径不加全局锁、不落库。
	svc.topics.inc(pk.TopicName)

	if cl.Net.Inline {
		return pk, nil
	}
	// 只有在有监控端在线时才记录消息流水与推送事件；常态运行时这条路径几乎零开销。
	if svc.hubCount.Load() == 0 {
		return pk, nil
	}
	l := model.MessageLog{
		TS:        time.Now(),
		ClientID:  cl.ID,
		Username:  string(cl.Properties.Username),
		Topic:     pk.TopicName,
		QoS:       int(pk.FixedHeader.Qos),
		Retained:  pk.FixedHeader.Retain,
		Size:      len(pk.Payload),
		Direction: "in",
	}
	svc.msgRing.add(l)
	msg := l
	svc.publishEvent(Event{Type: EventMessage, Timestamp: msg.TS, Message: &msg})
	return pk, nil
}
