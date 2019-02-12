package blast_test

import (
	"bytes"
	"regexp"
	"sync"
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

func NewLoggingReadWriteCloser(data string) *LoggingReadWriteCloser {
	return &LoggingReadWriteCloser{
		Buf: bytes.NewBufferString(data),
	}
}

type LoggingReadWriteCloser struct {
	Buf      *bytes.Buffer
	DidRead  bool
	DidWrite bool
	DidClose bool
}

func (l *LoggingReadWriteCloser) Read(p []byte) (n int, err error) {
	l.DidRead = true
	return l.Buf.Read(p)
}

func (l *LoggingReadWriteCloser) Write(p []byte) (n int, err error) {
	l.DidWrite = true
	return l.Buf.Write(p)
}

func (l *LoggingReadWriteCloser) Close() error {
	l.DidClose = true
	return nil
}

func (l *LoggingReadWriteCloser) mustClose(t *testing.T) {
	t.Helper()
	if !l.DidClose {
		t.Fatal("Did not close")
	}
}
func (l *LoggingReadWriteCloser) mustRead(t *testing.T) {
	t.Helper()
	if !l.DidRead {
		t.Fatal("Did not read")
	}
}
func (l *LoggingReadWriteCloser) mustWrite(t *testing.T) {
	t.Helper()
	if !l.DidWrite {
		t.Fatal("Did not write")
	}
}
func (l *LoggingReadWriteCloser) mustNotClose(t *testing.T) {
	t.Helper()
	if l.DidClose {
		t.Fatal("Did close")
	}
}
func (l *LoggingReadWriteCloser) mustNotRead(t *testing.T) {
	t.Helper()
	if l.DidRead {
		t.Fatal("Did read")
	}
}
func (l *LoggingReadWriteCloser) mustNotWrite(t *testing.T) {
	t.Helper()
	if l.DidWrite {
		t.Fatal("Did write")
	}
}