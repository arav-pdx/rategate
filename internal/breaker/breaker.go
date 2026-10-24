package breaker

import (
	errors"
	sync"
	time"
)

type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

var ErrBreakerOpen = errors.New("circuit breaker is open")

type CircuitBreaker struct {
	mu           sync.Mutex
	state        State
	failures     int
	threshold    int
	cooldown     time.Duration
	openedAt     time.Time
	successTrial int
	onStateHook  func(State)
}

func New(failureThreshold int, cooldown time.Duration, hook func(State)) *CircuitBreaker {
	return &CircuitBreaker{
		threshold:   failureThreshold,
		cooldown:    cooldown,
		state:       StateClosed,
		onStateHook: hook,
	}
}

func (cb *CircuitBreaker) Execute(fn func() error) error {
	cb.mu.Lock()
	now := time.Now()

	if cb.state == StateOpen {
		if now.Sub(cb.openedAt) > cb.cooldown {
			cb.state = StateHalfOpen
			cb.successTrial = 0
			if cb.onStateHook != nil {
				cb.onStateHook(StateHalfOpen)
			}
		} else {
			cb.mu.Unlock()
			return ErrBreakerOpen
		}
	}
	cb.mu.Unlock()

	err := fn()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failures++
		if cb.failures >= cb.threshold {
			cb.state = StateOpen
			cb.openedAt = time.Now()
			if cb.onStateHook != nil {
				cb.onStateHook(StateOpen)
			}
		}
		return err
	}

	if cb.state == StateHalfOpen {
		cb.successTrial++
		if cb.successTrial >= 3 {
			cb.state = StateClosed
			cb.failures = 0
			if cb.onStateHook != nil {
				cb.onStateHook(StateClosed)
			}
		}
	} else if cb.state == StateClosed {
		cb.failures = 0
	}

	return nil
}

func (cb *CircuitBreaker) State() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}
