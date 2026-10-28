package broker

import (
	"sync"

	"mqtt-proxy/internal/model"
)

// liveMessageRingSize 是内存消息环形缓冲的容量，仅用于监控页展示近期流水。
const liveMessageRingSize = 1000

// ---------------------------------------------------------------- 主题计数 --

// topicShardCount 是分片数，取 2 的幂便于用位与取模。
const topicShardCount = 32

// topicShard 是单个主题计数分片。
type topicShard struct {
	mu sync.Mutex
	m  map[string]int64
}

// topicTable 用分片锁替代发布热路径上的全局锁，降低高并发发布时的锁竞争。
type topicTable struct {
	shards [topicShardCount]topicShard
}

func newTopicTable() *topicTable {
	t := &topicTable{}
	for i := range t.shards {
		t.shards[i].m = map[string]int64{}
	}
	return t
}

// index 用 FNV-1a 32 位哈希把主题映射到某个分片（内联实现，避免额外分配）。
func (t *topicTable) index(topic string) uint32 {
	var h uint32 = 2166136261
	for i := 0; i < len(topic); i++ {
		h ^= uint32(topic[i])
		h *= 16777619
	}
	return h & (topicShardCount - 1)
}

// inc 对指定主题的计数 +1，只锁定所属分片。
func (t *topicTable) inc(topic string) {
	s := &t.shards[t.index(topic)]
	s.mu.Lock()
	s.m[topic]++
	s.mu.Unlock()
}

// snapshot 汇总所有分片，返回主题 -> 累计消息数（从启动至今）。
func (t *topicTable) snapshot() map[string]int {
	out := make(map[string]int)
	for i := range t.shards {
		s := &t.shards[i]
		s.mu.Lock()
		for k, v := range s.m {
			out[k] += int(v)
		}
		s.mu.Unlock()
	}
	return out
}

// ---------------------------------------------------------------- 消息环形 --

// msgRing 是固定容量的环形缓冲，保存最近的发布记录，不落库、供监控读取。
type msgRing struct {
	mu   sync.Mutex
	buf  []model.MessageLog
	next int  // 下一个写入位置（环形）
	full bool // 是否已开始覆盖最旧记录
}

func newMsgRing(size int) *msgRing {
	if size <= 0 {
		size = liveMessageRingSize
	}
	return &msgRing{buf: make([]model.MessageLog, 0, size)}
}

// add 追加一条消息，超出容量时覆盖最旧的一条。
func (r *msgRing) add(l model.MessageLog) {
	r.mu.Lock()
	if len(r.buf) < cap(r.buf) {
		r.buf = append(r.buf, l)
	} else {
		r.buf[r.next] = l
		r.next = (r.next + 1) % cap(r.buf)
		r.full = true
	}
	r.mu.Unlock()
}

// recent 返回最近 n 条消息，按时间从新到旧排列。
func (r *msgRing) recent(n int) []model.MessageLog {
	r.mu.Lock()
	defer r.mu.Unlock()
	if n <= 0 || len(r.buf) == 0 {
		return nil
	}
	if n > len(r.buf) {
		n = len(r.buf)
	}
	out := make([]model.MessageLog, 0, n)
	// 最新一条的索引：未满时是最后一个元素，满时是 next 前一个位置。
	idx := len(r.buf) - 1
	if r.full {
		idx = (r.next - 1 + cap(r.buf)) % cap(r.buf)
	}
	for i := 0; i < n; i++ {
		out = append(out, r.buf[idx])
		idx = (idx - 1 + cap(r.buf)) % cap(r.buf)
	}
	return out
}
