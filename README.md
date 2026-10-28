# mqtt-proxy

内嵌式多版本 MQTT Broker，自带 Web 管理后台。**单个自包含二进制**即可同时提供 MQTT 接入、管理 API 与管理界面——React/AntD 控制台在构建时通过 `go:embed` 内嵌进 Go 可执行文件，部署时无需任何前端产物或外部依赖。

## 特性

- **多版本 MQTT**：基于 [mochi-mqtt/server v2](https://github.com/mochi-mqtt/server)，同时支持 MQTT v3.1 / v3.1.1 / v5.0
- **双协议接入**：TCP（默认 `:1883`）与 WebSocket（默认 `:8083`）监听器
- **ACL 权限体系**：按 ClientID / 用户名 / 主题通配符（`+`、`#`）授权，支持发布/订阅/双向、allow/deny 效果、QoS 上限与优先级；可配置无匹配规则时默认放行或默认拒绝
- **匿名连接开关**：可允许未认证设备直接接入，或强制走账号 / 设备注册认证
- **实时监控**：连接数、订阅数、消息吞吐的时序曲线，消息流水、活跃主题统计、在线客户端明细，支持 WebSocket 实时推送与控制台代发消息
- **设备与主题台账**：注册客户端设备、管理主题清单，与 ACL 联动
- **纯 Go 存储**：SQLite（`modernc.org/sqlite`，无 CGO），单文件数据库，内置版本化迁移引擎
- **零配置引导**：首次启动未检测到配置文件时进入 Web 引导模式，只需设置 4 项基础配置即可运行
- **JWT 管理认证**：后台账号分角色（`superadmin` / `admin` / `readonly` / `mqtt`），bcrypt 存储口令，JWT 密钥引导时自动随机生成

## 技术栈

| 层 | 技术 |
|---|---|
| 后端 | Go 1.27 · Gin · mochi-mqtt v2.7.9 · golang-jwt/v5 · bcrypt · gorilla/websocket |
| 存储 | SQLite（modernc.org/sqlite，纯 Go）· 自研 embed SQL 迁移引擎 |
| 前端 | React 18 · Ant Design 5 · Vite 5 · TypeScript |
| 交付 | `go:embed` 内嵌前端，单二进制；交叉编译无 CGO 依赖 |

## 目录结构

```
mqtt-proxy/
├── main.go                 # 程序入口：引导检测、子命令、优雅退出
├── frontend.go             # go:embed 内嵌 admin/dist 前端产物
├── gen.go                  # go:generate 辅助脚本（前端构建 + 交叉编译）
├── build.ps1               # Windows 本机一键构建（前端 + 内嵌 exe）
├── Makefile                # Linux/通用构建入口
├── config.conf             # 运行时配置（首次启动由引导页生成）
├── internal/
│   ├── config/             # 配置加载：环境变量 > 配置文件 > 内置默认值
│   ├── app/                # 应用装配与生命周期、迁移子命令
│   ├── broker/             # mochi-mqtt 集成：监听器、认证、ACL、事件钩子
│   ├── api/                # Gin 路由：管理 API、引导 API、监控 WebSocket
│   ├── auth/               # JWT 签发校验、bcrypt、角色权限
│   ├── match/              # MQTT 主题通配符匹配（+ / #）
│   ├── migrate/            # embed SQL 迁移脚本与执行引擎
│   ├── db/                 # SQLite 连接与 pragma
│   ├── model/              # 数据模型（用户、ACL、设备、主题、监控快照）
│   ├── repo/               # 数据访问层
│   └── logging/            # 运行日志：按大小轮转、按天清理
├── admin/                  # 管理后台前端（React + AntD + Vite + TS）
│   └── src/pages/          # 登录 / 引导 / 仪表盘 / 监控 / ACL / 用户 / 客户端 / 主题
├── release/                # go generate 产出的发布二进制（已 gitignore）
└── data/                   # SQLite 数据库文件（运行时生成，已 gitignore）
```

## 快速开始

### 1. 构建

```powershell
# Windows 本机开发构建：产出 mqtt-proxy.exe（前端已内嵌）
.\build.ps1
```

```bash
# Linux/通用构建
make            # 等价于：admin 前端 npm run build + go build 内嵌二进制
```

```bash
# 发布构建（两步 go:generate，产物在 release/）
go generate .                 # 第一步编译前端，第二步交叉编译 Linux 二进制
go run gen.go windows         # 可选：追加产出 Windows 发布包
```

### 2. 首次启动：Web 引导

直接运行二进制。若当前目录（或 `-config` 指定路径）下没有配置文件，服务进入**引导模式**：

```powershell
.\mqtt-proxy.exe
# 控制台提示：未检测到配置文件 "config.conf"，服务以引导模式启动。
# 打开 http://localhost:8080 完成基础配置
```

引导页仅需设置 4 项，其余参数全部使用内置默认值：

| 配置项 | 说明 |
|---|---|
| 允许匿名连接 | 未携带账号密码的设备是否可直接接入 |
| 无匹配 ACL 规则时默认放行 | 关闭后，未命中任何 ACL 规则的收发将被拒绝 |
| 超级管理员用户名 | 管理后台登录账号（启动时自动 seed） |
| 超级管理员密码 | 至少 6 位，bcrypt 存储 |

保存后自动生成 `config.conf`（**JWT 密钥为随机生成的 32 字节十六进制值**），服务随即以新配置自动切换到正常模式，浏览器跳转登录页。

### 3. 使用管理后台

浏览器打开 `http://localhost:8080`，用引导时设置的超管账号登录：

- **仪表盘**：账号 / 规则 / 设备 / 主题总量与 Broker 运行指标一览
- **实时监控**：连接数、订阅数、消息吞吐曲线，消息流水、活跃主题、在线客户端，WebSocket 秒级推送，可代发测试消息
- **ACL 规则**：按客户端 / 用户 / 主题通配符授权，控制 QoS 上限、效果与优先级
- **客户端设备**：注册允许接入的设备 ClientID，可强制断开在线连接
- **用户**：管理后台与 MQTT 认证账号（角色：超级管理员 / 管理员 / 只读 / MQTT）
- **主题**：维护主题清单与说明

### 4. 设备接入

| 协议 | 默认地址 |
|---|---|
| MQTT over TCP | `<host>:1883`（v3.1 / v3.1.1 / v5.0） |
| MQTT over WebSocket | `ws://<host>:8083` |

匿名开启时设备可不带凭据接入；关闭后需使用后台创建的 MQTT 账号或已注册设备。收发行为受 ACL 规则与「默认放行」开关共同约束。

## 配置参考

取值优先级：**环境变量 > 配置文件（`config.conf`） > 内置默认值**。任何项留空或删除即回退默认值。

引导页生成的配置文件只包含以下键；其余键可按需手工添加：

| 键 | 默认值 | 说明 |
|---|---|---|
| `MQTT_ALLOW_ANONYMOUS` | `true` | 是否允许匿名连接 |
| `MQTT_DEFAULT_ACL_ALLOW` | `true` | 无匹配 ACL 规则时是否默认放行 |
| `JWT_SECRET` | 引导时随机生成 | 管理后台 JWT 签名密钥 |
| `ADMIN_USER` / `ADMIN_PASS` | — | 超级管理员账号（启动时 seed / 同步） |

其余内置默认值项（可用环境变量或手工写入配置文件覆盖）：

| 键 | 默认值 | 说明 |
|---|---|---|
| `HTTP_ADDR` | `:8080` | 管理后台 / API 监听地址 |
| `MQTT_TCP_ADDR` | `:1883` | MQTT TCP 监听地址 |
| `MQTT_WS_ADDR` | `:8083` | MQTT WebSocket 监听地址 |
| `MQTT_PROTOCOLS` | `3,4,5` | 接受的 MQTT 协议版本 |
| `DB_PATH` | `data/mqtt-proxy.db` | SQLite 数据库文件路径 |
| `JWT_TTL` | `12h` | 登录令牌有效期 |
| `MONITOR_INTERVAL` | `2s` | 监控采样间隔 |
| `LOG_FILE` | `server.log` | 运行日志路径（按大小轮转、按天清理） |
| `LOG_MAX_SIZE_MB` / `LOG_MAX_AGE_DAYS` | `2` / `5` | 单个日志大小上限 MB / 历史保留天数 |
| `LOG_STDOUT` / `LOG_LEVEL` | `true` / `info` | 是否同时输出控制台 / 日志级别 |

生成包含全部可调项的完整配置模板：

```bash
./mqtt-proxy gen-config [路径]     # 默认写到 config.conf，已存在则拒绝覆盖
```

## 命令行

```bash
mqtt-proxy                        # 启动服务（无配置文件时进入引导模式）
mqtt-proxy -config /path/to.conf  # 指定配置文件路径
mqtt-proxy gen-config [路径]      # 生成默认配置模板
mqtt-proxy migrate up             # 应用全部待执行迁移
mqtt-proxy migrate down           # 回滚最近一次迁移
mqtt-proxy migrate status         # 查看迁移状态（schema_migrations）
mqtt-proxy migrate force N        # 将版本 N 标记为已应用（修复中断场景）
```

用 systemd 部署 Linux 时，把 `release/mqtt-proxy-linux-amd64` 拷贝为目标机任意目录直接运行即可（无任何动态依赖），首次启动同样走 Web 引导流程。

## 安全建议

- 引导页自动生成的 `JWT_SECRET` 为强随机值，请勿替换为弱口令；泄露后需重启前更换并全员重新登录
- 生产环境建议关闭「允许匿名连接」，并将「无匹配 ACL 规则时默认放行」置为 `false`，用显式 ACL 白名单收敛权限
- `readonly` 角色可用于巡检岗位，只有 `superadmin` / `admin` 可写入
- 管理端口（8080）与 MQTT 端口（1883/8083）默认无 TLS，公网部署请置于反向代理 / 防火墙之后
