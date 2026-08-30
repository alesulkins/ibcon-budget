import type { CSSProperties, ReactNode } from 'react';

/**
 * Таблица «строка × месяцы» на всю доступную ширину.
 *
 * В проекте максимум 12 месяцев, поэтому колонки всегда помещаются:
 * таблица растягивается на 100% белой области, а при меньшем числе
 * месяцев ячейки становятся шире, а не таблица уже.
 *
 * `table-layout: fixed` обязателен: без него ширина колонки зависит от
 * содержимого, и строки с разным наполнением (например, дни командировок
 * под графиком) переставали совпадать по вертикали.
 */

export const monthGridTable: CSSProperties = {
  width: '100%',
  tableLayout: 'fixed',
  borderCollapse: 'collapse',
};

export const monthGridHeadCell: CSSProperties = {
  padding: '2px 4px',
  fontSize: 11,
  color: '#888',
  fontWeight: 400,
  textAlign: 'center',
  whiteSpace: 'nowrap',
};

export const monthGridCell: CSSProperties = {
  padding: '2px 4px',
  textAlign: 'center',
};

/** Ширина колонки-подписи слева (типы квартир, «по РФ» и т.п.). */
export const LABEL_COL_WIDTH = 120;

/**
 * Наименьшая ширина колонки месяца.
 *
 * На телефоне двенадцать колонок в 390 точек не помещаются никак: без
 * этого предела подписи месяцев наезжали друг на друга, а поля ввода
 * сжимались в полоску, куда нельзя ни попасть пальцем, ни прочитать
 * сумму. Поэтому таблица не ужимается ниже своего минимума, а
 * прокручивается вбок — столбцы месяцев остаются столбцами.
 */
const MIN_MONTH_COL = 62;

interface Props {
  /** Подписи месяцев по порядку. */
  months: string[];
  /** Ширина левой колонки-подписи; 0 — колонки нет. */
  labelWidth?: number;
  /** Дополнительные колонки справа (например «Итого») и их ширина. */
  trailingWidth?: number;
  head?: ReactNode;
  children: ReactNode;
}

/**
 * Задаёт сетку колонок через <colgroup>, чтобы все строки таблицы
 * гарантированно делили ширину одинаково.
 */
export default function MonthGrid({
  months, labelWidth = 0, trailingWidth = 0, head, children,
}: Props) {
  // Ширина, ниже которой таблица не сжимается. На широком экране она
  // меньше доступного места, и ничего не меняется: table растягивается
  // на 100%, как и раньше.
  const minWidth = labelWidth + trailingWidth + months.length * MIN_MONTH_COL;

  return (
    // ibcon-grid — разлиновка строк из index.css: каждая строка отделена
    // той же линией, что и шапки блоков.
    <div style={{ overflowX: 'auto' }} className="ibcon-scroll">
      <table style={{ ...monthGridTable, minWidth }} className="ibcon-grid">
        <colgroup>
          {labelWidth > 0 && <col style={{ width: labelWidth }} />}
          {months.map((_, i) => <col key={i} />)}
          {trailingWidth > 0 && <col style={{ width: trailingWidth }} />}
        </colgroup>
        {head}
        {children}
      </table>
    </div>
  );
}
