import { Tooltip } from 'antd';
import { fmtPct } from '../utils/fmt';
import { profitabilityGrade } from '../utils/profitability';
import { FONT_NUM } from '../theme';

interface Props {
  value: number | null | undefined;
  /** Крупнее и жирнее — для карточек и шапок, а не для ячейки таблицы. */
  strong?: boolean;
}

/**
 * Рентабельность в цвете по единой шкале. Подсказка расшифровывает
 * оттенок словами: по цвету «жёлто-оранжевый против красно-оранжевого»
 * на глаз не различить.
 */
export default function Profitability({ value, strong }: Props) {
  if (value == null) return <span>—</span>;

  const { color, label } = profitabilityGrade(value);

  return (
    <Tooltip title={label}>
      <span style={{
        color,
        fontWeight: strong ? 700 : 600,
        fontSize: strong ? 16 : undefined,
        whiteSpace: 'nowrap',
        fontFamily: FONT_NUM,
      }}>
        {fmtPct(value)}
      </span>
    </Tooltip>
  );
}
