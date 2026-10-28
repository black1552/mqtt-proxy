// Package repo 在 SQLite 句柄之上提供数据访问。每个 repo 都是一层
// 显式的 SQL 封装，不使用 ORM，便于审计查询语句。
package repo

import (
	"database/sql"
	"errors"
	"strings"

	"mqtt-proxy/internal/model"
)

// ErrNotFound 在查询无结果行时返回。
var ErrNotFound = errors.New("not found")

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Store 将所有仓库绑定到同一个数据库句柄上。
type Store struct {
	DB *sql.DB

	Users  *UserRepo
	ACL    *ACLRepo
	Clients *ClientRepo
	Topics *TopicRepo
	Stats  *StatsRepo
}

// NewStore 将所有仓库装配到 db 上。
func NewStore(db *sql.DB) *Store {
	return &Store{
		DB:      db,
		Users:   &UserRepo{db: db},
		ACL:     &ACLRepo{db: db},
		Clients: &ClientRepo{db: db},
		Topics:  &TopicRepo{db: db},
		Stats:   &StatsRepo{db: db},
	}
}

// ------------------------------------------------------------------ users ---

// UserRepo 管理管理员与 MQTT 凭证账号。
type UserRepo struct{ db *sql.DB }

const userCols = "id, username, password_hash, role, enabled, remark, created_at, updated_at"

func scanUser(row interface{ Scan(...any) error }) (model.User, error) {
	var u model.User
	var enabled int
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &enabled, &u.Remark, &u.CreatedAt, &u.UpdatedAt)
	u.Enabled = enabled == 1
	return u, err
}

func (r *UserRepo) GetByUsername(username string) (model.User, error) {
	return scanUser(r.db.QueryRow("SELECT "+userCols+" FROM users WHERE username = ?", username))
}

func (r *UserRepo) Get(id int64) (model.User, error) {
	return scanUser(r.db.QueryRow("SELECT "+userCols+" FROM users WHERE id = ?", id))
}

func (r *UserRepo) List(includeMQTT bool, query string) ([]model.User, error) {
	var sb strings.Builder
	sb.WriteString("SELECT " + userCols + " FROM users WHERE 1=1")
	args := []any{}
	if !includeMQTT {
		sb.WriteString(" AND role != 'mqtt'")
	}
	if query != "" {
		sb.WriteString(" AND username LIKE ?")
		args = append(args, "%"+query+"%")
	}
	sb.WriteString(" ORDER BY id ASC")
	rows, err := r.db.Query(sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (r *UserRepo) Create(u *model.User) error {
	res, err := r.db.Exec(
		"INSERT INTO users (username, password_hash, role, enabled, remark) VALUES (?,?,?,?,?)",
		u.Username, u.PasswordHash, u.Role, boolToInt(u.Enabled), u.Remark)
	if err != nil {
		return err
	}
	u.ID, _ = res.LastInsertId()
	return nil
}

func (r *UserRepo) Update(u *model.User) error {
	_, err := r.db.Exec(
		"UPDATE users SET username=?, role=?, enabled=?, remark=?, password_hash=?, updated_at=CURRENT_TIMESTAMP WHERE id=?",
		u.Username, u.Role, boolToInt(u.Enabled), u.Remark, u.PasswordHash, u.ID)
	return err
}

func (r *UserRepo) UpdatePassword(id int64, hash string) error {
	_, err := r.db.Exec("UPDATE users SET password_hash=?, updated_at=CURRENT_TIMESTAMP WHERE id=?", hash, id)
	return err
}

func (r *UserRepo) Delete(id int64) error {
	_, err := r.db.Exec("DELETE FROM users WHERE id=?", id)
	return err
}

func (r *UserRepo) Count() (int, error) {
	var n int
	err := r.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&n)
	return n, err
}

// ------------------------------------------------------------------- acl ----

// ACLRepo 管理访问控制规则。
type ACLRepo struct{ db *sql.DB }

const aclCols = "id, name, client_id, username, topic, access, effect, max_qos, priority, enabled, created_at, updated_at"

func scanACL(row interface{ Scan(...any) error }) (model.ACLRule, error) {
	var a model.ACLRule
	var enabled int
	err := row.Scan(&a.ID, &a.Name, &a.ClientID, &a.Username, &a.Topic, &a.Access, &a.Effect,
		&a.MaxQoS, &a.Priority, &enabled, &a.CreatedAt, &a.UpdatedAt)
	a.Enabled = enabled == 1
	return a, err
}

func (r *ACLRepo) List(query string) ([]model.ACLRule, error) {
	var sb strings.Builder
	sb.WriteString("SELECT " + aclCols + " FROM acl_rules WHERE 1=1")
	args := []any{}
	if query != "" {
		sb.WriteString(" AND (name LIKE ? OR topic LIKE ?)")
		args = append(args, "%"+query+"%", "%"+query+"%")
	}
	sb.WriteString(" ORDER BY priority ASC, id ASC")
	rows, err := r.db.Query(sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.ACLRule
	for rows.Next() {
		a, err := scanACL(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// EnabledRules 返回 Broker 评估所用的规则集，按优先级排序。
func (r *ACLRepo) EnabledRules() ([]model.ACLRule, error) {
	rows, err := r.db.Query("SELECT " + aclCols + " FROM acl_rules WHERE enabled = 1 ORDER BY priority ASC, id ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.ACLRule
	for rows.Next() {
		a, err := scanACL(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *ACLRepo) Get(id int64) (model.ACLRule, error) {
	return scanACL(r.db.QueryRow("SELECT "+aclCols+" FROM acl_rules WHERE id = ?", id))
}

func (r *ACLRepo) Create(a *model.ACLRule) error {
	res, err := r.db.Exec(
		`INSERT INTO acl_rules (name, client_id, username, topic, access, effect, max_qos, priority, enabled)
         VALUES (?,?,?,?,?,?,?,?,?)`,
		a.Name, a.ClientID, a.Username, a.Topic, a.Access, a.Effect, a.MaxQoS, a.Priority, boolToInt(a.Enabled))
	if err != nil {
		return err
	}
	a.ID, _ = res.LastInsertId()
	return nil
}

func (r *ACLRepo) Update(a *model.ACLRule) error {
	_, err := r.db.Exec(
		`UPDATE acl_rules SET name=?, client_id=?, username=?, topic=?, access=?, effect=?, max_qos=?, priority=?, enabled=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		a.Name, a.ClientID, a.Username, a.Topic, a.Access, a.Effect, a.MaxQoS, a.Priority, boolToInt(a.Enabled), a.ID)
	return err
}

func (r *ACLRepo) Delete(id int64) error {
	_, err := r.db.Exec("DELETE FROM acl_rules WHERE id=?", id)
	return err
}

func (r *ACLRepo) Count() (int, error) {
	var n int
	err := r.db.QueryRow("SELECT COUNT(*) FROM acl_rules").Scan(&n)
	return n, err
}

// --------------------------------------------------------------- clients ----

// ClientRepo 管理已登记的客户端设备。
type ClientRepo struct{ db *sql.DB }

const clientCols = "id, name, description, enabled, created_at, updated_at"

func scanClient(row interface{ Scan(...any) error }) (model.ClientDevice, error) {
	var c model.ClientDevice
	var enabled int
	err := row.Scan(&c.ID, &c.Name, &c.Description, &enabled, &c.CreatedAt, &c.UpdatedAt)
	c.Enabled = enabled == 1
	return c, err
}

func (r *ClientRepo) List(query string) ([]model.ClientDevice, error) {
	var sb strings.Builder
	sb.WriteString("SELECT " + clientCols + " FROM clients WHERE 1=1")
	args := []any{}
	if query != "" {
		sb.WriteString(" AND (id LIKE ? OR name LIKE ?)")
		args = append(args, "%"+query+"%", "%"+query+"%")
	}
	sb.WriteString(" ORDER BY created_at DESC")
	rows, err := r.db.Query(sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.ClientDevice
	for rows.Next() {
		c, err := scanClient(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *ClientRepo) Get(id string) (model.ClientDevice, error) {
	return scanClient(r.db.QueryRow("SELECT "+clientCols+" FROM clients WHERE id = ?", id))
}

func (r *ClientRepo) Upsert(c *model.ClientDevice) error {
	_, err := r.db.Exec(
		`INSERT INTO clients (id, name, description, enabled) VALUES (?,?,?,?)
         ON CONFLICT(id) DO UPDATE SET name=excluded.name, description=excluded.description, updated_at=CURRENT_TIMESTAMP`,
		c.ID, c.Name, c.Description, boolToInt(c.Enabled))
	return err
}

func (r *ClientRepo) Update(c *model.ClientDevice) error {
	_, err := r.db.Exec(
		"UPDATE clients SET name=?, description=?, enabled=?, updated_at=CURRENT_TIMESTAMP WHERE id=?",
		c.Name, c.Description, boolToInt(c.Enabled), c.ID)
	return err
}

func (r *ClientRepo) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM clients WHERE id=?", id)
	return err
}

func (r *ClientRepo) Count() (int, error) {
	var n int
	err := r.db.QueryRow("SELECT COUNT(*) FROM clients").Scan(&n)
	return n, err
}

// ----------------------------------------------------------------- topics ---

// TopicRepo 管理主题目录条目。
type TopicRepo struct{ db *sql.DB }

const topicCols = "id, topic, description, enabled, created_at, updated_at"

func scanTopic(row interface{ Scan(...any) error }) (model.Topic, error) {
	var t model.Topic
	var enabled int
	err := row.Scan(&t.ID, &t.Topic, &t.Description, &enabled, &t.CreatedAt, &t.UpdatedAt)
	t.Enabled = enabled == 1
	return t, err
}

func (r *TopicRepo) List(query string) ([]model.Topic, error) {
	var sb strings.Builder
	sb.WriteString("SELECT " + topicCols + " FROM topics WHERE 1=1")
	args := []any{}
	if query != "" {
		sb.WriteString(" AND topic LIKE ?")
		args = append(args, "%"+query+"%")
	}
	sb.WriteString(" ORDER BY topic ASC")
	rows, err := r.db.Query(sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Topic
	for rows.Next() {
		t, err := scanTopic(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *TopicRepo) Get(id int64) (model.Topic, error) {
	return scanTopic(r.db.QueryRow("SELECT "+topicCols+" FROM topics WHERE id = ?", id))
}

func (r *TopicRepo) Create(t *model.Topic) error {
	res, err := r.db.Exec("INSERT INTO topics (topic, description, enabled) VALUES (?,?,?)",
		t.Topic, t.Description, boolToInt(t.Enabled))
	if err != nil {
		return err
	}
	t.ID, _ = res.LastInsertId()
	return nil
}

func (r *TopicRepo) Update(t *model.Topic) error {
	_, err := r.db.Exec("UPDATE topics SET topic=?, description=?, enabled=?, updated_at=CURRENT_TIMESTAMP WHERE id=?",
		t.Topic, t.Description, boolToInt(t.Enabled), t.ID)
	return err
}

func (r *TopicRepo) Delete(id int64) error {
	_, err := r.db.Exec("DELETE FROM topics WHERE id=?", id)
	return err
}

func (r *TopicRepo) Count() (int, error) {
	var n int
	err := r.db.QueryRow("SELECT COUNT(*) FROM topics").Scan(&n)
	return n, err
}

// ------------------------------------------------------------------ stats ---

// StatsRepo 持久化监控快照与近期消息环形记录。
type StatsRepo struct{ db *sql.DB }

func (r *StatsRepo) AddSnapshot(s model.StatsSnapshot) error {
	_, err := r.db.Exec(
		`INSERT INTO stats_history (ts, clients_connected, subscriptions, messages_received, messages_sent, retained)
         VALUES (?,?,?,?,?,?)`,
		s.TS.UTC(), s.ClientsConnected, s.Subscriptions, s.MessagesReceived, s.MessagesSent, s.Retained)
	return err
}

// RecentSnapshots 返回最近 n 条快照，按时间从旧到新排序。
func (r *StatsRepo) RecentSnapshots(n int) ([]model.StatsSnapshot, error) {
	rows, err := r.db.Query(
		`SELECT id, ts, clients_connected, subscriptions, messages_received, messages_sent, retained
         FROM stats_history ORDER BY id DESC LIMIT ?`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.StatsSnapshot
	for rows.Next() {
		var s model.StatsSnapshot
		if err := rows.Scan(&s.ID, &s.TS, &s.ClientsConnected, &s.Subscriptions, &s.MessagesReceived, &s.MessagesSent, &s.Retained); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	// 反转为从旧到新，便于绘制图表
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, rows.Err()
}

func (r *StatsRepo) AddMessageLog(l model.MessageLog) error {
	_, err := r.db.Exec(
		`INSERT INTO message_logs (ts, client_id, username, topic, qos, retained, size, direction)
         VALUES (?,?,?,?,?,?,?,?)`,
		l.TS.UTC(), l.ClientID, l.Username, l.Topic, l.QoS, boolToInt(l.Retained), l.Size, l.Direction)
	return err
}

func (r *StatsRepo) RecentMessages(n int) ([]model.MessageLog, error) {
	rows, err := r.db.Query(
		`SELECT id, ts, client_id, username, topic, qos, retained, size, direction
         FROM message_logs ORDER BY id DESC LIMIT ?`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.MessageLog
	for rows.Next() {
		var l model.MessageLog
		var retained int
		if err := rows.Scan(&l.ID, &l.TS, &l.ClientID, &l.Username, &l.Topic, &l.QoS, &retained, &l.Size, &l.Direction); err != nil {
			return nil, err
		}
		l.Retained = retained == 1
		out = append(out, l)
	}
	return out, rows.Err()
}

// TrimMessageLogs 仅保留最新的 keep 行，以限制表增长。
func (r *StatsRepo) TrimMessageLogs(keep int) error {
	_, err := r.db.Exec(
		`DELETE FROM message_logs WHERE id NOT IN (SELECT id FROM message_logs ORDER BY id DESC LIMIT ?)`, keep)
	return err
}

// TrimSnapshots 仅保留最新的 keep 行。
func (r *StatsRepo) TrimSnapshots(keep int) error {
	_, err := r.db.Exec(
		`DELETE FROM stats_history WHERE id NOT IN (SELECT id FROM stats_history ORDER BY id DESC LIMIT ?)`, keep)
	return err
}
