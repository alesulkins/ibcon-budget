-- Справочник: исполнители
CREATE TABLE executors (
    id   SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    full_name VARCHAR(255) NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by INTEGER
);

-- Справочник: должности
CREATE TABLE positions (
    id   SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by INTEGER
);

-- Справочник: режим работы
CREATE TABLE work_modes (
    id        SERIAL PRIMARY KEY,
    code      VARCHAR(50) NOT NULL UNIQUE,
    full_name VARCHAR(255) NOT NULL,
    active    BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by INTEGER
);

-- Справочник: статьи затрат
CREATE TABLE cost_items (
    id           SERIAL PRIMARY KEY,
    name         VARCHAR(255) NOT NULL UNIQUE,
    is_calculated BOOLEAN NOT NULL DEFAULT FALSE,
    active       BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order   INTEGER NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by   INTEGER
);

-- Пользователи
CREATE TABLE users (
    id            SERIAL PRIMARY KEY,
    email         VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    full_name     VARCHAR(255) NOT NULL,
    role          VARCHAR(50) NOT NULL,        -- GE, EP, IP, RP, AP, MANAGEMENT
    active        BOOLEAN NOT NULL DEFAULT TRUE,
    failed_attempts INTEGER NOT NULL DEFAULT 0,
    locked_until    TIMESTAMPTZ,
    last_activity   TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by    INTEGER REFERENCES users(id),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Права доступа к проектам (индивидуальные)
CREATE TABLE user_project_permissions (
    id          SERIAL PRIMARY KEY,
    user_id     INTEGER NOT NULL REFERENCES users(id),
    project_id  INTEGER NOT NULL,              -- FK добавим после создания таблицы projects
    can_edit    BOOLEAN NOT NULL DEFAULT FALSE,
    granted_by  INTEGER NOT NULL REFERENCES users(id),
    granted_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, project_id)
);

-- Проекты
CREATE TABLE projects (
    id            SERIAL PRIMARY KEY,
    name          VARCHAR(255) NOT NULL,
    customer      VARCHAR(255) NOT NULL,
    executor_id   INTEGER NOT NULL REFERENCES executors(id),
    location      VARCHAR(255) NOT NULL,
    start_date    DATE NOT NULL,
    duration_months INTEGER NOT NULL,
    end_date      DATE NOT NULL,
    director      VARCHAR(255) NOT NULL,
    manager       VARCHAR(255) NOT NULL,
    administrator VARCHAR(255) NOT NULL,
    economist     VARCHAR(255) NOT NULL,
    status        VARCHAR(50) NOT NULL DEFAULT 'prospect',
    -- prospect | active | suspended | completed | unrealized
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by    INTEGER NOT NULL REFERENCES users(id),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by    INTEGER REFERENCES users(id)
);

ALTER TABLE user_project_permissions
    ADD CONSTRAINT fk_upp_project FOREIGN KEY (project_id) REFERENCES projects(id);

-- Бюджеты (один бюджет на проект, несколько версий)
CREATE TABLE budgets (
    id           SERIAL PRIMARY KEY,
    project_id   INTEGER NOT NULL UNIQUE REFERENCES projects(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by   INTEGER NOT NULL REFERENCES users(id)
);

-- Версии бюджета
CREATE TABLE budget_versions (
    id            SERIAL PRIMARY KEY,
    budget_id     INTEGER NOT NULL REFERENCES budgets(id),
    version_label VARCHAR(20),                  -- NULL для черновика/на согл/согласован; в.1, в.2 для архива
    status        VARCHAR(50) NOT NULL DEFAULT 'draft',
    -- draft | under_review | approved | archive
    comment       TEXT,
    cost_no_vat   NUMERIC(18,2),                -- кэш G236
    profitability NUMERIC(10,4),                -- кэш G244
    cost_override NUMERIC(18,2),                -- ручная корректировка стоимости
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by    INTEGER NOT NULL REFERENCES users(id),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by    INTEGER REFERENCES users(id),
    approved_at   TIMESTAMPTZ,
    copied_from   INTEGER REFERENCES budget_versions(id)
);

-- Исходные данные бюджета (помесячные, по категориям)
-- Хранится как JSON для гибкости: каждый лист = один тип данных
CREATE TABLE budget_inputs (
    id                SERIAL PRIMARY KEY,
    budget_version_id INTEGER NOT NULL REFERENCES budget_versions(id),
    input_type        VARCHAR(100) NOT NULL,   -- employees, rent_apartments, transport, ...
    data              JSONB NOT NULL DEFAULT '{}',
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by        INTEGER REFERENCES users(id),
    UNIQUE(budget_version_id, input_type)
);

-- Рассчитанные результаты (строки 232-249 листа 2.Бюджет)
CREATE TABLE budget_results (
    id                SERIAL PRIMARY KEY,
    budget_version_id INTEGER NOT NULL UNIQUE REFERENCES budget_versions(id),
    total_expenses    NUMERIC(18,2),    -- G232
    op_margin_pct     NUMERIC(10,4),    -- % операционной маржинальности
    total_revenue     NUMERIC(18,2),    -- G236
    op_profit         NUMERIC(18,2),    -- G234/G238
    tax               NUMERIC(18,2),    -- G240
    net_profit        NUMERIC(18,2),    -- G242
    profitability     NUMERIC(10,4),    -- G244
    revenue_with_vat  NUMERIC(18,2),    -- G247
    ref_rate_amount   NUMERIC(18,2),    -- G249
    monthly_data      JSONB,            -- все помесячные итоги
    calculated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- История изменений (audit log)
CREATE TABLE audit_log (
    id          BIGSERIAL PRIMARY KEY,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    user_id     INTEGER REFERENCES users(id),
    user_role   VARCHAR(50),
    action      VARCHAR(100) NOT NULL,
    object_type VARCHAR(100),
    object_id   INTEGER,
    comment     TEXT
);

CREATE INDEX idx_audit_log_occurred_at ON audit_log(occurred_at DESC);
CREATE INDEX idx_audit_log_user_id ON audit_log(user_id);
CREATE INDEX idx_budget_versions_budget_id ON budget_versions(budget_id);
CREATE INDEX idx_budget_versions_status ON budget_versions(status);
CREATE INDEX idx_projects_status ON projects(status);
