-- Оклады по городам. Сами города остаются: записи справочников удалять
-- нельзя (CLAUDE.md, правило 3), а таблица создана этой же миграцией —
-- откат её сносит только вместе с окладами.
DROP TABLE IF EXISTS position_city_salaries;
DROP TABLE IF EXISTS cities;
