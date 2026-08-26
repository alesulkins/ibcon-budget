-- Порядковый номер версии бюджета внутри проекта.
--
-- Раньше номер жил только в version_label и присваивался лишь при уходе
-- версии в архив («в.1»), поэтому у черновиков в таблице стояли прочерки.
-- Теперь номер получает каждая версия при создании.
ALTER TABLE budget_versions ADD COLUMN IF NOT EXISTS version_no INTEGER;

-- Проставляем номера уже существующим версиям: по порядку создания
-- внутри каждого проекта (budget_id → project_id).
WITH numbered AS (
    SELECT bv.id,
           ROW_NUMBER() OVER (PARTITION BY b.project_id ORDER BY bv.created_at, bv.id) AS n
    FROM budget_versions bv
    JOIN budgets b ON b.id = bv.budget_id
)
UPDATE budget_versions bv
SET version_no = numbered.n
FROM numbered
WHERE numbered.id = bv.id
  AND bv.version_no IS NULL;

ALTER TABLE budget_versions ALTER COLUMN version_no SET NOT NULL;

-- Личный кабинет: аватар и рабочие заметки пользователя.
--
-- avatar хранит либо эмодзи (одна-две «буквы»), либо data:-URL
-- загруженной картинки, поэтому тип текстовый без ограничения длины.
ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS notes  TEXT NOT NULL DEFAULT '';
