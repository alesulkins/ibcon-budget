-- Откат справочных значений. Должности, добранные вверх, не удаляем:
-- записи справочников удалять запрещено (CLAUDE.md, правило 3).
ALTER TABLE executors DROP COLUMN IF EXISTS refinancing_rate;
ALTER TABLE executors DROP COLUMN IF EXISTS profit_tax_rate;
ALTER TABLE positions DROP COLUMN IF EXISTS salary;
