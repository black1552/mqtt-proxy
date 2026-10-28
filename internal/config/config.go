// Package config 集中管理 MQTT 代理的运行时配置。
// 配置来源的优先级为：环境变量 > 配置文件 > 内置默认值，
// 使二进制既能零配置直接部署，也能通过配置文件精细调整。
package config

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"strconv"
	"strings"
	"time"
)

// DefaultConfigFile 是未显式指定时尝试读取的配置文件名。
const DefaultConfigFile = "config.conf"

// Config 包含 Broker 与管理 API 用到的所有可调参数。
type Config struct {
	// ConfigFile 是本次启动尝试读取的配置文件路径；
	// 文件不存在时服务进入引导模式，由 Web 页面生成该文件。
	ConfigFile string

	// HTTP 管理服务监听地址。
	HTTPAddr string

	// MQTT 监听器。
	MQTTTCPAddr string
	MQTTWSAddr  string

	// 存储。
	DBPath string

	// 后台 / 认证。
	JWTSecret      string
	JWTTTL         time.Duration
	AdminUser      string
	AdminPass      string
	AllowAnonymous bool

	// Broker 行为。
	DefaultACLAllow bool
	// AcceptProtocolVersions 列出 Broker 接受的 MQTT 协议版本。
	// 3 = MQTT 3.1，4 = MQTT 3.1.1，5 = MQTT 5.0。
	AcceptProtocolVersions []byte

	// 监控。
	MonitorInterval time.Duration

	// 运行日志（写入 server.log 并按大小轮转、按天清理）。
	LogFile       string // 日志文件路径
	LogMaxSizeMB  int    // 单个日志文件最大 MB，超过即轮转
	LogMaxAgeDays int    // 历史日志保留天数，超过即删除
	LogStdout     bool   // 是否同时输出到标准输出
	LogLevel      string // 日志级别：debug/info/warn/error
}

// Load 按「环境变量 > 配置文件 > 默认值」的优先级构建 Config。
func Load() *Config {
	path := getenvDefault("CONFIG_FILE", DefaultConfigFile)
	s := source{file: parseFile(path)}

	cfg := &Config{
		ConfigFile:      path,
		HTTPAddr:        s.str("HTTP_ADDR", ":8080"),
		MQTTTCPAddr:     s.str("MQTT_TCP_ADDR", ":1883"),
		MQTTWSAddr:      s.str("MQTT_WS_ADDR", ":8083"),
		DBPath:          s.str("DB_PATH", "data/mqtt-proxy.db"),
		JWTSecret:       s.str("JWT_SECRET", "mqtt-proxy-insecure-dev-secret-change-me"),
		JWTTTL:          s.dur("JWT_TTL", 12*time.Hour),
		AdminUser:       s.str("ADMIN_USER", "superadmin"),
		AdminPass:       s.str("ADMIN_PASS", "854512"),
		AllowAnonymous:  s.bool("MQTT_ALLOW_ANONYMOUS", true),
		DefaultACLAllow: s.bool("MQTT_DEFAULT_ACL_ALLOW", true),
		MonitorInterval: s.dur("MONITOR_INTERVAL", 2*time.Second),
		LogFile:         s.str("LOG_FILE", "server.log"),
		LogMaxSizeMB:    s.int("LOG_MAX_SIZE_MB", 2),
		LogMaxAgeDays:   s.int("LOG_MAX_AGE_DAYS", 5),
		LogStdout:       s.bool("LOG_STDOUT", true),
		LogLevel:        s.str("LOG_LEVEL", "info"),
	}
	cfg.AcceptProtocolVersions = s.bytes("MQTT_PROTOCOLS", []byte{3, 4, 5})
	return cfg
}

// source 封装一个配置取值器：先查环境变量，再查配置文件，最后回退默认值。
type source struct {
	file map[string]string
}

func (s source) lookup(key string) (string, bool) {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v, true
	}
	if v, ok := s.file[key]; ok {
		if v = strings.TrimSpace(v); v != "" {
			return v, true
		}
	}
	return "", false
}

func (s source) str(key, def string) string {
	if v, ok := s.lookup(key); ok {
		return v
	}
	return def
}

func (s source) bool(key string, def bool) bool {
	if v, ok := s.lookup(key); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func (s source) int(key string, def int) int {
	if v, ok := s.lookup(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func (s source) dur(key string, def time.Duration) time.Duration {
	if v, ok := s.lookup(key); ok {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

// bytes 解析形如 "3,4,5" 的逗号分隔整数列表。
func (s source) bytes(key string, def []byte) []byte {
	v, ok := s.lookup(key)
	if !ok {
		return def
	}
	var out []byte
	for _, part := range strings.Split(v, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return def
		}
		out = append(out, byte(n))
	}
	if len(out) == 0 {
		return def
	}
	return out
}

// parseFile 读取 KEY=VALUE 形式的配置文件；文件不存在时返回空表。
// 支持以 # 开头的整行注释、可选的 export 前缀以及值两侧成对引号。
func parseFile(path string) map[string]string {
	m := map[string]string{}
	data, err := os.ReadFile(path)
	if err != nil {
		return m
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, "\"'")
		if key != "" {
			m[key] = val
		}
	}
	return m
}

// DefaultTemplate 是供 gen-config 写出的默认配置模板内容。
const DefaultTemplate = `# mqtt-proxy 配置模板
# 取值优先级：环境变量 > 本配置文件 > 内置默认值。
# 以 # 开头的行为注释；某项留空或删除即回退到内置默认值。

# --- HTTP 管理后台监听地址 ---
HTTP_ADDR=:8080

# --- MQTT 监听地址（TCP 与 WebSocket）---
MQTT_TCP_ADDR=:1883
MQTT_WS_ADDR=:8083

# 接受的 MQTT 协议版本，逗号分隔：3=3.1 4=3.1.1 5=5.0
MQTT_PROTOCOLS=3,4,5

# 是否允许匿名连接；无匹配 ACL 规则时是否默认放行
MQTT_ALLOW_ANONYMOUS=true
MQTT_DEFAULT_ACL_ALLOW=true

# --- 存储（SQLite 数据库文件路径）---
DB_PATH=data/mqtt-proxy.db

# --- 认证 / 后台账号 ---
JWT_SECRET=mqtt-proxy-insecure-dev-secret-change-me
JWT_TTL=12h
ADMIN_USER=superadmin
ADMIN_PASS=854512

# --- 监控 ---
MONITOR_INTERVAL=2s

# --- 运行日志（写入文件并按大小轮转、按天清理）---
# 日志文件路径
LOG_FILE=server.log
# 单个日志文件最大 MB，超过即轮转为历史文件
LOG_MAX_SIZE_MB=2
# 历史日志保留天数，超过即自动删除
LOG_MAX_AGE_DAYS=5
# 是否同时输出到标准输出（控制台）
LOG_STDOUT=true
# 日志级别：debug/info/warn/error
LOG_LEVEL=info
`

// Exists 报告本次启动读取的配置文件是否存在。
func (c *Config) Exists() bool {
	if c.ConfigFile == "" {
		return false
	}
	_, err := os.Stat(c.ConfigFile)
	return err == nil
}

// BootstrapInput 是引导模式（首次启动、无配置文件）下由 Web 页面提交的基础运行配置。
// 仅包含需要人工决定的项目，其余参数一律使用内置默认值，JWT 密钥在保存时自动生成。
type BootstrapInput struct {
	AllowAnonymous  bool
	DefaultACLAllow bool
	AdminUser       string
	AdminPass       string
}

// WriteConfigFile 将引导配置写入 path（KEY=VALUE 形式），
// 供服务重启后按「环境变量 > 配置文件 > 默认值」正常加载。
// 未写出的配置项在加载时自动回退到内置默认值；JWT 密钥每次随机生成。
func WriteConfigFile(path string, in BootstrapInput) error {
	var b strings.Builder
	b.WriteString("# mqtt-proxy 运行时配置（由 Web 引导页在首次启动时生成）\n")
	b.WriteString("# 取值优先级：环境变量 > 本配置文件 > 内置默认值。\n")
	b.WriteString("# 未列出的参数（监听地址、数据库路径、日志等）均使用内置默认值。\n\n")
	writeKV(&b, "是否允许匿名连接", "MQTT_ALLOW_ANONYMOUS", strconv.FormatBool(in.AllowAnonymous))
	writeKV(&b, "无匹配 ACL 规则时是否默认放行", "MQTT_DEFAULT_ACL_ALLOW", strconv.FormatBool(in.DefaultACLAllow))
	writeKV(&b, "认证 / 后台账号（JWT 密钥为保存时自动生成的随机值）", "JWT_SECRET", randomSecret())
	writeKV(&b, "", "ADMIN_USER", in.AdminUser)
	writeKV(&b, "", "ADMIN_PASS", in.AdminPass)
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// writeKV 写出一行配置，comment 非空时在其上方输出注释行。
func writeKV(b *strings.Builder, comment, key, value string) {
	if comment != "" {
		b.WriteString("# --- ")
		b.WriteString(comment)
		b.WriteString(" ---\n")
	}
	b.WriteString(key)
	b.WriteString("=")
	b.WriteString(value)
	b.WriteString("\n\n")
}

// randomSecret 生成 32 字节随机十六进制字符串，用作 JWT 密钥。
func randomSecret() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "mqtt-proxy-insecure-dev-secret-change-me"
	}
	return hex.EncodeToString(buf)
}

// WriteTemplate 将默认配置模板写入 path（若文件已存在则拒绝覆盖）。
func WriteTemplate(path string) error {
	if _, err := os.Stat(path); err == nil {
		return &existsError{path: path}
	}
	return os.WriteFile(path, []byte(DefaultTemplate), 0o644)
}

// existsError 表示目标文件已存在。
type existsError struct{ path string }

func (e *existsError) Error() string {
	return "配置文件已存在，未做覆盖: " + e.path
}

func getenvDefault(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
