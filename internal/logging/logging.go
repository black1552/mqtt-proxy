// Package logging 提供一个不依赖第三方库的运行日志方案：
// 按文件大小轮转、按保留天数清理历史文件，并装配标准库 slog。
package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RotatingWriter 是一个并发安全的 io.Writer：
//   - 当前活动文件写到 maxSize 字节后轮转为带时间戳的历史文件；
//   - 每次轮转及启动时清理修改时间超过 maxAge 的历史文件。
type RotatingWriter struct {
	path     string
	maxSize  int64
	maxAge   time.Duration
	now      func() time.Time

	mu   sync.Mutex
	f    *os.File
	size int64
}

// NewRotating 构造轮转写入器，并立即清理一次超期历史文件。
func NewRotating(path string, maxSize int64, maxAge time.Duration) *RotatingWriter {
	w := &RotatingWriter{path: path, maxSize: maxSize, maxAge: maxAge, now: time.Now}
	w.prune()
	return w
}

// Write 实现 io.Writer。写入前若将超出大小限制则先轮转。
func (w *RotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.f == nil {
		if err := w.open(); err != nil {
			return 0, err
		}
	}
	// 单条日志本身就超过上限时，仍写入以避免丢日志（此时文件可能略超 maxSize）。
	if w.size+int64(len(p)) > w.maxSize && w.size > 0 {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}
	n, err := w.f.Write(p)
	w.size += int64(n)
	return n, err
}

// Close 关闭底层文件句柄。
func (w *RotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f != nil {
		err := w.f.Close()
		w.f = nil
		return err
	}
	return nil
}

func (w *RotatingWriter) open() error {
	if dir := filepath.Dir(w.path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	w.f = f
	w.size = st.Size()
	return nil
}

// rotate 关闭并把活动文件改名为历史文件，随后新建活动文件并清理超期项。
func (w *RotatingWriter) rotate() error {
	if w.f != nil {
		_ = w.f.Close()
		w.f = nil
	}
	dest := w.path + "." + w.now().Format("2006-01-02T15-04-05.000")
	// 极端情况下同一毫秒内多次轮转，追加序号保证唯一。
	for i := 1; ; i++ {
		if _, err := os.Stat(dest); os.IsNotExist(err) {
			break
		}
		dest = dest + "." + strconv.Itoa(i)
	}
	if err := os.Rename(w.path, dest); err != nil {
		return err
	}
	if err := w.open(); err != nil {
		return err
	}
	w.prune()
	return nil
}

// prune 删除同级目录中以 "<path>." 为前缀、且修改时间超过 maxAge 的历史文件。
func (w *RotatingWriter) prune() {
	dir := filepath.Dir(w.path)
	base := filepath.Base(w.path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name == base || !strings.HasPrefix(name, base+".") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if w.now().Sub(info.ModTime()) > w.maxAge {
			_ = os.Remove(filepath.Join(dir, name))
		}
	}
}

// Config 描述日志装配所需参数。
type Config struct {
	File       string
	MaxSizeMB  int
	MaxAgeDays int
	Stdout     bool
	Level      string
}

// New 依据配置构建 slog.Logger 与底层轮转写入器。
// 调用方应在进程退出前 Close 返回的写入器。
func New(c Config) (*slog.Logger, *RotatingWriter) {
	maxSize := int64(2) << 20
	if c.MaxSizeMB > 0 {
		maxSize = int64(c.MaxSizeMB) << 20
	}
	maxAge := 5 * 24 * time.Hour
	if c.MaxAgeDays > 0 {
		maxAge = time.Duration(c.MaxAgeDays) * 24 * time.Hour
	}

	w := NewRotating(c.File, maxSize, maxAge)

	var out io.Writer = w
	if c.Stdout {
		out = io.MultiWriter(w, os.Stdout)
	}
	logger := slog.New(slog.NewTextHandler(out, &slog.HandlerOptions{Level: parseLevel(c.Level)}))
	return logger, w
}

// parseLevel 将字符串映射为 slog.Level，无法识别时回退 Info。
func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
