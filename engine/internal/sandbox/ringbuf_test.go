package sandbox

import (
	"strings"
	"testing"
)

func TestRingBuffer_StoresShortWritesVerbatim(t *testing.T) {
	rb := newRingBuffer(1024)
	rb.WriteString("hello ")
	rb.WriteString("world")

	if got := rb.String(); got != "hello world" {
		t.Errorf("short writes: got %q", got)
	}
	if rb.Truncated() {
		t.Error("did not expect truncation")
	}
}

func TestRingBuffer_EvictsOldestBytesOverCap(t *testing.T) {
	rb := newRingBuffer(10)
	rb.WriteString("0123456789ABCDEF") // 16 bytes into a 10-byte ring

	// Tail retention — last 10 bytes are kept; the prefix marker
	// tells the reader content was dropped.
	got := rb.String()
	if !strings.Contains(got, "6789ABCDEF") {
		t.Errorf("tail bytes missing: %q", got)
	}
	if !strings.Contains(got, "truncated") {
		t.Errorf("truncation marker missing: %q", got)
	}
	if !rb.Truncated() {
		t.Error("Truncated() should report true after overflow")
	}
}

func TestRingBuffer_SingleWriteLargerThanCapKeepsTail(t *testing.T) {
	rb := newRingBuffer(8)
	rb.WriteString(strings.Repeat("X", 50) + "FINAL")

	got := rb.String()
	if !strings.HasSuffix(got, "FINAL") {
		t.Errorf("final bytes lost: got %q", got)
	}
}

func TestRingBuffer_NoTruncationMarkerAtExactCap(t *testing.T) {
	rb := newRingBuffer(5)
	rb.WriteString("ABCDE") // exactly 5 bytes
	if rb.Truncated() {
		t.Error("exact-cap write should not trip Truncated()")
	}
	if rb.String() != "ABCDE" {
		t.Errorf("verbatim retention failed: %q", rb.String())
	}
}

func TestRingBuffer_ConcurrentWritesDoNotRace(t *testing.T) {
	// The race detector is the primary assertion — go test -race must
	// not flag any concurrent access to the underlying slice.
	rb := newRingBuffer(256)
	done := make(chan struct{})
	for i := 0; i < 8; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				rb.WriteString("xxxx") // 4 bytes per write
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < 8; i++ {
		<-done
	}
	// 8 goroutines × 100 writes × 4 bytes = 3,200 bytes through a 256-byte
	// ring. Truncated must be true (any positive count of evictions).
	if !rb.Truncated() {
		t.Error("Truncated should be true after 3,200 bytes through a 256-byte ring")
	}
}
