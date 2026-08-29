-- Напоминания в личном кабинете и персональные настройки интерфейса.

-- Напоминание — заметка с датой: в срок она всплывает уведомлением на
-- экране, а при включённой почте ещё и уходит письмом.
CREATE TABLE reminders (
    id        SERIAL PRIMARY KEY,
    user_id   INTEGER NOT NULL REFERENCES users(id),
    text      TEXT NOT NULL,
    remind_at TIMESTAMPTZ NOT NULL,

    -- Отметки «уже сработало». Показ на экране и письмо разведены: почту
    -- можно выключить, и тогда emailed_at так и останется пустым, а
    -- shown_at заполнится. Одно поле «сработало» этого не различало бы.
    shown_at   TIMESTAMPTZ,
    emailed_at TIMESTAMPTZ,

    -- done — пользователь закрыл напоминание как выполненное. Записи не
    -- удаляем: список сделанного — тоже история работы.
    done       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Выборка «что пора показать этому пользователю» идёт по владельцу и
-- сроку, поэтому индекс составной.
CREATE INDEX idx_reminders_user_due ON reminders(user_id, remind_at);

-- Тумблер общий на все напоминания, а не на каждое: настройка доставки,
-- а не свойство записи. Включён по умолчанию — почта была бы бесполезна,
-- если бы её приходилось включать после каждого напоминания.
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_reminders BOOLEAN NOT NULL DEFAULT TRUE;

-- Настройки интерфейса: размер шрифта, тема, цвета. Хранятся в учётке, а
-- не в браузере, чтобы человек видел свой интерфейс на любом устройстве.
-- JSONB, потому что набор настроек будет расти, а миграция на каждый
-- новый переключатель — лишняя.
ALTER TABLE users ADD COLUMN IF NOT EXISTS ui_settings JSONB NOT NULL DEFAULT '{}'::jsonb;
