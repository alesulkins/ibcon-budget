package reminders

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"
	"time"
)

// Отправка напоминаний почтой.
//
// SMTP настраивается переменными окружения. Если хост не задан —
// отправка ВЫКЛЮЧЕНА целиком: письма не уходят, напоминания продолжают
// всплывать на экране. Это рабочее состояние, а не поломка: почтовый
// сервер подключат отдельно, а до тех пор платформа не должна ни падать,
// ни делать вид, что письмо ушло.

// SMTPConfig — параметры почтового сервера.
type SMTPConfig struct {
	Host string
	Port string
	User string
	Pass string
	// From — адрес отправителя. Пусто — берётся User.
	From string
}

// Enabled — настроен ли сервер. Без хоста отправлять некуда.
func (c SMTPConfig) Enabled() bool { return strings.TrimSpace(c.Host) != "" }

func (c SMTPConfig) from() string {
	if strings.TrimSpace(c.From) != "" {
		return c.From
	}
	return c.User
}

// Mailer шлёт письма напоминаний.
type Mailer struct {
	cfg SMTPConfig
}

func NewMailer(cfg SMTPConfig) *Mailer { return &Mailer{cfg: cfg} }

// Send отправляет одно напоминание. Возвращает ошибку, если сервер
// настроен, но письмо не ушло: тогда напоминание не помечается
// отправленным и попадёт в следующую попытку.
func (m *Mailer) Send(to, fullName, text string, remindAt time.Time) error {
	if !m.cfg.Enabled() {
		return fmt.Errorf("почтовый сервер не настроен")
	}

	subject := "Напоминание — IBCON Бюджет"
	body := fmt.Sprintf(
		"%s, напоминание на %s:\r\n\r\n%s\r\n\r\n—\r\nIBCON Бюджет",
		firstName(fullName), remindAt.Format("02.01.2006 15:04"), text)

	// Тема письма русская, поэтому кодируем её по RFC 2047: без этого
	// почтовые клиенты показали бы кракозябры.
	msg := "From: " + m.cfg.from() + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + encodeHeader(subject) + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"\r\n" + body

	addr := m.cfg.Host + ":" + m.cfg.Port
	var auth smtp.Auth
	if m.cfg.User != "" {
		auth = smtp.PlainAuth("", m.cfg.User, m.cfg.Pass, m.cfg.Host)
	}
	return smtp.SendMail(addr, auth, m.cfg.from(), []string{to}, []byte(msg))
}

// encodeHeader — заголовок письма в кодировке base64 по RFC 2047.
func encodeHeader(s string) string {
	return "=?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte(s)) + "?="
}

func firstName(fullName string) string {
	parts := strings.Fields(fullName)
	if len(parts) >= 2 {
		return parts[1] // «Фамилия Имя Отчество» → имя
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return "Здравствуйте"
}

// Worker периодически рассылает наступившие напоминания.
//
// Опрос, а не расписание на каждое напоминание: сроки редкие, а
// планировщик на тысячу таймеров пришлось бы восстанавливать после
// каждого перезапуска.
type Worker struct {
	svc    *Service
	mailer *Mailer
	every  time.Duration
}

func NewWorker(svc *Service, mailer *Mailer, every time.Duration) *Worker {
	return &Worker{svc: svc, mailer: mailer, every: every}
}

// Start запускает рассылку в фоне. Если SMTP не настроен, воркер не
// стартует вовсе — крутить пустой цикл незачем.
func (w *Worker) Start() {
	if !w.mailer.cfg.Enabled() {
		slog.Info("напоминания: SMTP не настроен, письма не отправляются")
		return
	}
	go func() {
		t := time.NewTicker(w.every)
		defer t.Stop()
		for range t.C {
			w.tick()
		}
	}()
}

// batchSize ограничивает пачку: при накопившемся хвосте рассылка идёт
// частями, а не одним долгим заходом.
const batchSize = 50

func (w *Worker) tick() {
	pending, err := w.svc.PendingEmails(batchSize)
	if err != nil {
		slog.Error("напоминания: не удалось выбрать письма", "err", err)
		return
	}
	for _, p := range pending {
		if err := w.mailer.Send(p.Email, p.FullName, p.Text, p.RemindAt); err != nil {
			// Не помечаем отправленным: попробуем в следующий заход.
			slog.Error("напоминания: письмо не отправлено",
				"reminder_id", p.ID, "err", err)
			continue
		}
		if err := w.svc.MarkEmailed(p.ID); err != nil {
			slog.Error("напоминания: не удалось отметить отправку",
				"reminder_id", p.ID, "err", err)
		}
	}
}
