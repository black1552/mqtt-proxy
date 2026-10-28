-- 0002_observability.up.sql：支撑实时监控的相关表。

-- 定期对 Broker 计数器做的快照，用于绘制趋势图。
CREATE TABLE stats_history (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    ts                TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    clients_connected INTEGER NOT NULL DEFAULT 0,
    subscriptions     INTEGER NOT NULL DEFAULT 0,
    messages_received INTEGER NOT NULL DEFAULT 0,
    messages_sent     INTEGER NOT NULL DEFAULT 0,
    retained          INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_stats_ts ON stats_history (ts);

-- 近期发布活动的有界环形记录，展示在实时控制台上。
CREATE TABLE message_logs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    ts          TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    client_id   TEXT NOT NULL DEFAULT '',
    username    TEXT NOT NULL DEFAULT '',
    topic       TEXT NOT NULL,
    qos         INTEGER NOT NULL DEFAULT 0,
    retained    INTEGER NOT NULL DEFAULT 0,
    size        INTEGER NOT NULL DEFAULT 0,
    direction   TEXT NOT NULL DEFAULT 'in'  -- in | out
);
CREATE INDEX idx_msglog_ts ON message_logs (ts);
