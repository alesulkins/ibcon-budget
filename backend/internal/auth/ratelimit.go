package auth

import (
	"sync"
	"time"
)

// Ограничение частоты попыток входа.
const (
	loginAttemptsPerWindow = 5
	loginWindow            = time.Minute
)

type loginCounter struct {
	count     int
	windowEnd time.Time
}

// RateLimiter — счётчики попыток входа по ключам.
type RateLimiter struct {
	mu       sync.Mutex
	counters map[string]*loginCounter
	// lastPrune — когда в последний раз выбрасывали протухшие счётчики.
	// Без уборки карта росла бы на каждый новый адрес источника.
	lastPrune time.Time
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{counters: make(map[string]*loginCounter)}
}

// Allow говорит, можно ли обрабатывать попытку входа. Второе значение —
// сколько ждать до следующей возможной попытки. Считаются только НЕУДАЧНЫЕ
// попытки (см.
func (l *RateLimiter) Allow(keys ...string) (bool, time.Duration) {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	l.prune(now)

	for _, k := range keys {
		c := l.counters[k]
		if c != nil && now.Before(c.windowEnd) && c.count >= loginAttemptsPerWindow {
			return false, c.windowEnd.Sub(now)
		}
	}
	return true, 0
}

// Fail отмечает неудачную попытку входа по каждому ключу.
func (l *RateLimiter) Fail(keys ...string) {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, k := range keys {
		c := l.counters[k]
		if c == nil || !now.Before(c.windowEnd) {
			c = &loginCounter{windowEnd: now.Add(loginWindow)}
			l.counters[k] = c
		}
		c.count++
	}
}

// Reset обнуляет счётчики после удачного входа: человек, вспомнивший
// пароль с третьей попытки, не должен доживать минуту с чужим счётчиком.
func (l *RateLimiter) Reset(keys ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, k := range keys {
		delete(l.counters, k)
	}
}

func (l *RateLimiter) prune(now time.Time) {
	if now.Sub(l.lastPrune) < loginWindow {
		return
	}
	l.lastPrune = now
	for k, c := range l.counters {
		if !now.Before(c.windowEnd) {
			delete(l.counters, k)
		}
	}
}
