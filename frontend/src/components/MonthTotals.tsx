import { Typography } from 'antd';
import { fmtNum } from '../utils/fmt';
import MonthGrid, { monthGridCell, monthGridHeadCell } from './MonthGrid';

const { Text } = Typography;

const LABEL_COL = 160;
const TOTAL_COL = 130;

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
            <th style={{ ...monthGridHeadCell, textAlign: 'left', color: '#333', fontWeight: 500 }}>
              {label}
            </th>
            {months.map((m, i) => <th key={i} style={monthGridHeadCell}>{m}</th>)}
            <th style={{ ...monthGridHeadCell, textAlign: 'right', color: '#333', fontWeight: 500 }}>
              Итого
            </th>
          </tr>
        </thead>
      )}
    >
      <tbody>
        <tr style={{ background: '#fafafa' }}>
          <td style={{ ...monthGridCell, textAlign: 'left' }}>
            <Text type="secondary" style={{ fontSize: 12 }}>расход месяца</Text>
          </td>
          {months.map((_, i) => (
            <td key={i} style={{ ...monthGridCell, fontSize: 12, fontWeight: 500 }}>
              {values[i] ? fmtNum(values[i]) : '—'}
            </td>
          ))}
          <td style={{ ...monthGridCell, textAlign: 'right', fontSize: 12, fontWeight: 700 }}>
            {fmtNum(total)}
          </td>
        </tr>
      </tbody>
    </MonthGrid>
  );
}
