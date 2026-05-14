package sandbox

import "sync"

// truncationMarker is prepended to the buffer's stringified output once
// any bytes have been evicted from the head. The text is short enough
// to fit comfortably in any reasonable cap and obvious enough that a
// human scanning the log knows content is missing.
const truncationMarker = "[... output truncated ...]\n"

// ringBuffer is a fixed-capacity, append-only byte buffer with tail
// retention. Writes that would overflow `cap` evict bytes from the
// head; the most recent `cap` bytes are always preserved. This is the
// right policy for sandbox logs because errors and final agent
// summaries cluster at the end of the run.
//
// The collector wraps it under a mutex (outputCollector.mu) for the
// production write path; ringBuffer has its own internal mutex so a
// direct, lock-free caller cannot accidentally observe a torn write.
type ringBuffer struct {
	mu        sync.Mutex
	cap       int
	chunks    []byte
	truncated bool
}

// newRingBuffer returns a ringBuffer with the given byte cap. A
// non-positive cap is a programming error — panic at construction
// rather than silently discard every write. Unit tests pass small but
// positive caps to exercise eviction logic; the only invalid input is
// cap <= 0, which would mean "discard all output", an operational
// nightmare we never want in production.
func newRingBuffer(cap int) *ringBuffer {
	if cap <= 0 {
		panic("sandbox.newRingBuffer: cap must be > 0")
	}
	return &ringBuffer{cap: cap}
}

// WriteString appends s to the buffer. If the result would exceed cap,
// the oldest bytes are dropped to keep len(chunks) <= cap.
func (r *ringBuffer) WriteString(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.chunks = append(r.chunks, s...)
	if len(r.chunks) > r.cap {
		excess := len(r.chunks) - r.cap
		r.chunks = r.chunks[excess:]
		r.truncated = true
	}
}

// String returns the retained content. If any bytes have been evicted
// the truncationMarker is prepended so consumers know they are looking
// at the tail rather than the whole stream.
func (r *ringBuffer) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.truncated {
		return truncationMarker + string(r.chunks)
	}
	return string(r.chunks)
}

// Truncated reports whether the buffer has ever evicted bytes. Useful
// for the grader / orchestrator to decide whether to surface a
// "transcript may be incomplete" warning to operators.
func (r *ringBuffer) Truncated() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.truncated
}
