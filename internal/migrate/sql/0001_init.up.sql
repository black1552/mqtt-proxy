-- 0001_init.up.sql：MQTT 代理管理面的核心数据库结构。

CREATE TABLE users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    username      TEXT    NOT NULL UNIQUE,
    password_hash TEXT    NOT NULL,
    role          TEXT    NOT NULL DEFAULT 'mqtt',   -- superadmin | admin | readonly | mqtt
    enabled       INTEGER NOT NULL DEFAULT 1,
    remark        TEXT    NOT NULL DEFAULT '',
    created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE acl_rules (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL,
    client_id   TEXT NOT NULL DEFAULT '',   -- 空=任意客户端 id（支持 + / # 通配）
    username    TEXT NOT NULL DEFAULT '',   -- 空=任意用户名
    topic       TEXT NOT NULL,              -- 规则适用的主题过滤器（支持 + / #）
    access      TEXT NOT NULL DEFAULT 'both', -- publish | subscribe | both
    effect      TEXT NOT NULL DEFAULT 'allow',-- allow | deny
    max_qos     INTEGER NOT NULL DEFAULT 2,
    priority    INTEGER NOT NULL DEFAULT 100, -- 值越小优先级越高
    enabled     INTEGER NOT NULL DEFAULT 1,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_acl_enabled_priority ON acl_rules (enabled, priority);

CREATE TABLE clients (
    id          TEXT PRIMARY KEY,           -- mqtt 客户端 id
    name        TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    enabled     INTEGER NOT NULL DEFAULT 1, -- 为 false 时 Broker 拒绝该客户端 id 连接
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE topics (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    topic       TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    enabled     INTEGER NOT NULL DEFAULT 1,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
