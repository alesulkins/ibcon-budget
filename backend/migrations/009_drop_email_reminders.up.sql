-- Рассылка напоминаний письмом отменена (решение владельца 2026-08-30):
-- напоминания всплывают уведомлением на экране. Тумблер доставки и
-- отметка об отправке больше ничего не значат.
ALTER TABLE users DROP COLUMN IF EXISTS email_reminders;
ALTER TABLE reminders DROP COLUMN IF EXISTS emailed_at;
