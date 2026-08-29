-- Оклад должности зависит от города: в разных городах за одну и ту же
-- работу платят по-разному, и в мастер проекта должен подставляться
-- оклад ГОРОДА ПРОЕКТА (projects.location).

-- Города — свой справочник: список пополняется прямо в форме должности,
-- поэтому свободной строкой в окладах его держать нельзя (опечатка
-- создала бы «второй Петербург» с отдельными окладами).
CREATE TABLE cities (
    id         SERIAL PRIMARY KEY,
    name       VARCHAR(255) NOT NULL UNIQUE,
    active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by INTEGER
);

INSERT INTO cities (name) VALUES ('Санкт-Петербург');

-- Оклад должности в конкретном городе.
--
-- Каскадное удаление только по должности: записи справочников удалять
-- нельзя (CLAUDE.md, правило 3), но если должность когда-нибудь удалят
-- миграцией, её оклады не должны остаться сиротами. Город же
-- деактивируется, а не удаляется — на него ссылка обычная.
CREATE TABLE position_city_salaries (
    position_id INTEGER NOT NULL REFERENCES positions(id) ON DELETE CASCADE,
    city_id     INTEGER NOT NULL REFERENCES cities(id),
    salary      NUMERIC(18,2) NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by  INTEGER,
    PRIMARY KEY (position_id, city_id)
);

CREATE INDEX idx_position_city_salaries_city ON position_city_salaries(city_id);

-- Уже заданные оклады переносим на Санкт-Петербург: другого города в
-- справочнике не было, значит они относились к нему.
--
-- positions.salary НЕ удаляем: это оклад «по умолчанию» — он подставится
-- там, где для города проекта своей ставки не задали, и остаётся
-- значением для городов, которые добавят позже.
INSERT INTO position_city_salaries (position_id, city_id, salary)
SELECT p.id, c.id, p.salary
FROM positions p
CROSS JOIN cities c
WHERE c.name = 'Санкт-Петербург'
  AND p.salary <> 0;
