package logging

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestRotate 验证：写入超过大小限制后会轮转出历史文件，且活动文件不超过上限。
func TestRotate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.log")
	w := NewRotating(path, 100, 24*time.Hour)
	defer w.Close()

	chunk := bytes.Repeat([]byte("x"), 40)
	for i := 0; i < 5; i++ {
		if _, err := w.Write(chunk); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) < 2 {
		t.Fatalf("期望轮转生成历史文件，实际目录项 %d 个", len(entries))
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Size() > 100 {
		t.Fatalf("活动文件 %d 字节，超过上限 100", st.Size())
	}
}

// TestPrune 验证：超过保留期的历史文件被清理，未超期的保留。
func TestPrune(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.log")

	old := filepath.Join(dir, "server.log.2020-01-01T00-00-00.000")
	if err := os.WriteFile(old, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	stale := time.Now().Add(-10 * 24 * time.Hour)
	if err := os.Chtimes(old, stale, stale); err != nil {
		t.Fatal(err)
	}

	fresh := filepath.Join(dir, "server.log.2026-01-01T00-00-00.000")
	if err := os.WriteFile(fresh, []byte("fresh"), 0o644); err != nil {
		t.Fatal(err)
	}

	w := NewRotating(path, 100, 5*24*time.Hour)
	defer w.Close()

	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatal("超期历史文件应被清理")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatal("未超期历史文件应被保留")
	}
}
