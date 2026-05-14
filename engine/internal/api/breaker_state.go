package api

import "expvar"

// graderBreakerStateValue reads the current circuit breaker state from
// the expvar var the experiment package publishes. The api package
// cannot import experiment (cycle), so it reads the named expvar
// directly. Returns "" if the var has not been initialized yet
// (only happens before the experiment package's init runs, which is
// effectively never in production).
func graderBreakerStateValue() string {
	v := expvar.Get("frameval_grader_breaker_state")
	if v == nil {
		return ""
	}
	if s, ok := v.(*expvar.String); ok {
		return s.Value()
	}
	return ""
}
