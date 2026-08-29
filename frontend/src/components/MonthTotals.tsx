import { Typography } from 'antd';
import { fmtNum } from '../utils/fmt';
import MonthGrid, { monthGridCell, monthGridHeadCell } from './MonthGrid';
import { FONT_NUM } from '../theme';

const { Text } = Typography;

const LABEL_COL = 160;
const TOTAL_COL = 130;

/**
 * Оформление строки помесячных итогов, вынесенное отдельно: тот же вид
 * нужен в подвале таблиц, которые строят сетку месяцев сами (листы-списки
 * 4.8/4.9/4.11). Эталон вида — лист 4.7.
 */
/**
 * Строка «расход месяца» — ЦВЕТ ПОДЛОЖКИ МЕНЯЕТСЯ ЗДЕСЬ.
 *
 * Темнее фона рабочей области, а не белее: итог должен читаться как
 * подведённая черта под таблицей, а не как ещё одна строка ввода.
 * Больше число в rgba — темнее полоса.
 */
export const totalsRow: React.CSSProperties = {
  background: 'rgba(24, 62, 77, 0.07)',
  borderTop: '2px solid rgba(24, 62, 77, 0.18)',
};

export const totalsLabelCell: React.CSSProperties = {
  ...monthGridCell,
  textAlign: 'left',
  fontSize: 12,
  fontWeight: 600,
};

export const totalsValueCell: React.CSSProperties = {
  ...monthGridCell,
  fontSize: 12,
  fontWeight: 500,
  fontFamily: FONT_NUM,
};

export const totalsGrandCell: React.CSSProperties = {
  ...monthGridCell,
  textAlign: 'right',
  fontSize: 12,
  fontWeight: 700,
  fontFamily: FONT_NUM,
};

interface Props {
  /** Подписи месяцев проекта по порядку. */
  months: string[];
  /** Суммы по месяцам; длина должна совпадать с months. */
  values: number[];
  /** Подпись строки слева. */
  label: string;
}

/**
 * Строка «сумма по каждому месяцу проекта», только для чтения.
 *
 * Нужна там, где ввод устроен не по месяцам, а расход всё равно надо
 * увидеть в помесячной раскладке: таблица покупок 4.7 и 4.12 (строки
 * привязаны к месяцу, но колонки таблицы — не месяцы) и лист 4.10, где
 * экономист задаёт два числа, а месяцы заполняются сами.
 */
export default function MonthTotals({ months, values, label }: Props) {
  const total = values.reduce((s, v) => s + (v || 0), 0);

  return (
    <MonthGrid
      months={months}
      labelWidth={LABEL_COL}
      trailingWidth={TOTAL_COL}
      head={(
        <thead>
          <tr>
            <th style={{ ...monthGridHeadCell, textAlign: 'left', color: 'var(--ibcon-text)', fontWeight: 500 }}>
              {label}
            </th>
            {months.map((m, i) => <th key={i} style={monthGridHeadCell}>{m}</th>)}
            <th style={{ ...monthGridHeadCell, textAlign: 'right', color: 'var(--ibcon-text)', fontWeight: 500 }}>
              Итого
            </th>
          </tr>
        </thead>
      )}
    >
      <tbody>
        <tr style={totalsRow}>
          <td style={{ ...totalsLabelCell, fontWeight: 400 }}>
            <Text type="secondary" style={{ fontSize: 12 }}>расход месяца</Text>
          </td>
          {months.map((_, i) => (
            <td key={i} style={totalsValueCell}>
              {values[i] ? fmtNum(values[i]) : '—'}
            </td>
          ))}
          <td style={totalsGrandCell}>{fmtNum(total)}</td>
        </tr>
      </tbody>
    </MonthGrid>
  );
}
