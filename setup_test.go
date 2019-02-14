package blitzkrieg_test

import (
	"bytes"
	"io"
	"regexp"
	"sync"
	"sync/atomic"
	"testing"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func mustMatch(t *testing.T, buf *ThreadSafeBuffer, num int, pattern string) {
	t.Helper()
	matches := regexp.MustCompile(pattern).FindAllString(buf.String(), -1)
	if len(matches) != num {
		t.Fatalf("Matches in output (%d) not expected (%d) for pattern %s:\n%s",
			len(matches),
			num,
			pattern,
			buf.String(),
		)
	}
}

//********************************************************************
// Utilities
//********************************************************************

type DummyCloser struct{}

func (DummyCloser) Close() error { return nil }

type ThreadSafeBuffer struct {
	b bytes.Buffer
	m sync.Mutex
}

func (b *ThreadSafeBuffer) Read(p []byte) (n int, err error) {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.Read(p)
}
func (b *ThreadSafeBuffer) Write(p []byte) (n int, err error) {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.Write(p)
}
func (b *ThreadSafeBuffer) String() string {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.String()
}

// NewWriteCounter returns a new io.Writer wrapper
// the passed in writer, where it counts total
// writes it has received.
func NewWriteCounter(w io.Writer) *WriteCounter {
	return &WriteCounter{w: w}
}

type WriteCounter struct {
	wl              sync.Mutex
	w               io.Writer
	bytesWritten    int64
	totalWriteCalls int64
}

func (w *WriteCounter) TotalWriteCalls() int {
	return int(atomic.LoadInt64(&w.totalWriteCalls))
}

func (w *WriteCounter) BytesWritten() int {
	return int(atomic.LoadInt64(&w.bytesWritten))
}

func (w *WriteCounter) Write(b []byte) (int, error) {
	atomic.AddInt64(&w.totalWriteCalls, 1)

	w.wl.Lock()
	written, err := w.w.Write(b)
	w.wl.Unlock()

	atomic.AddInt64(&w.bytesWritten, int64(written))
	return written, err
}
