package auth

import (
	"sync"
	"time"
)

// Ограничение частоты попыток входа.
//
// Без него подбор пароля упирался только в блокировку учётки, а она
// сама играла на руку нападающему: пять неверных паролей подряд — и
// настоящий человек не войдёт пятнадцать минут; повторяя это, чужую
// учётку можно держать закрытой сколько угодно. Ограничение по частоте
// останавливает перебор ДО счётчика учётки: автомат упирается в отказ
// на пятой попытке в минуту и до блокировки просто не доходит.
//
// Считаем по двум ключам сразу — по адресу источника и по почте:
// первый ловит перебор паролей к одной учётке, второй — перебор учёток
// с одного адреса и попытки блокировать чужую почту из разных сетей.
//
// Память: счётчики живут в процессе, не в базе. Проверка частоты
// переживать перезапуск не обязана, а вот лишний запрос к базе на
// каждую попытку входа — обязан не появиться.
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
// сколько ждать до следующей возможной попытки.
//
// Считаются только НЕУДАЧНЫЕ попытки (см. Fail): десять человек за одним
// внешним адресом входят одновременно и никому не мешают, а перебор
// паролей упирается в отказ с пятой ошибки.
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
