-- Индивидуальные права доступа сверх роли (гибридная модель, ТЗ 3.2).
--
-- Базовые права даёт роль (матрица в internal/auth/permissions.go).
-- Здесь лежит только то, что главный экономист выдал конкретному
-- пользователю ДОПОЛНИТЕЛЬНО.
--
-- project_id IS NULL — право действует на все проекты.
-- permission = '*'   — все права (кроме управления пользователями).
-- Две «звёздочки» дают два тумблера панели выдачи прав:
--   «это право на все проекты»  → (permission = X,   project_id = NULL)
--   «все права на один проект»  → (permission = '*', project_id = N)
CREATE TABLE IF NOT EXISTS user_permissions (
    id         SERIAL PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    permission VARCHAR(50) NOT NULL,
    project_id INTEGER REFERENCES projects(id),
    granted_by INTEGER NOT NULL REFERENCES users(id),
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Уникальность двумя частичными индексами, а не UNIQUE(...) по трём
-- колонкам: в SQL NULL не равен NULL, поэтому обычный UNIQUE пропустил
-- бы сколько угодно одинаковых глобальных прав.
CREATE UNIQUE INDEX IF NOT EXISTS uq_user_permissions_project
    ON user_permissions (user_id, permission, project_id)
    WHERE project_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_user_permissions_global
    ON user_permissions (user_id, permission)
    WHERE project_id IS NULL;

CREATE INDEX IF NOT EXISTS idx_user_permissions_user ON user_permissions(user_id);

-- Учётка может существовать без роли: ТЗ требует, чтобы после создания
-- пользователь не видел функциональности до назначения роли. Пустая
-- строка в role — это «роль не назначена».
ALTER TABLE users ALTER COLUMN role SET DEFAULT '';
