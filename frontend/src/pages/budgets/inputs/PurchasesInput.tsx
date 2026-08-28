import { useCallback, useEffect, useState } from 'react';
import { Card, Typography } from 'antd';
import { useQuery } from '@tanstack/react-query';
import { budgetsApi } from '../../../api';
import type { InputPurchases, ItemPurchase } from '../../../types';
import { monthLabel, fmtNum } from '../../../utils/fmt';
import PurchaseTable from '../../../components/PurchaseTable';
import MonthTotals from '../../../components/MonthTotals';
import { titleWithHint } from '../../../components/InfoHint';
import { useAutosave } from '../../../hooks/useAutosave';

const { Text } = Typography;

interface Props {
  versionId: number;
  /** Ключ ввода: equipment_items или corporate_events_items. */
  type: string;
  title: string;
  duration: number;
  startDate?: string;
  readonly?: boolean;
  /** Заголовок колонки описания; null — колонки нет (корпоративы). */
  nameLabel?: string | null;
  namePlaceholder?: string;
  monthColLabel: string;
  priceLabel: string;
  countLabel: string;
  addLabel: string;
  hint: string;
}

/**
 * Форма листа-покупок: строки с месяцем, количеством и ценой за единицу.
 * Расход месяца — сумма «цена × количество» по строкам этого месяца.
 *
 * Общая для 4.7 (приборы стройконтроля) и 4.12 (корпоративы). Доступны все
 * месяцы проекта, включая последние, — как и у вагончиков (4.4).
 */
export default function PurchasesInput({
  versionId, type, title, duration, startDate, readonly,
  nameLabel, namePlaceholder, monthColLabel, priceLabel, countLabel,
  addLabel, hint,
}: Props) {
  const [items, setItems] = useState<ItemPurchase[]>([]);
  const [hydrated, setHydrated] = useState(false);

  const { data: saved, isSuccess } = useQuery({
    queryKey: ['budget-input', versionId, type],
    queryFn: () => budgetsApi.getInput<InputPurchases>(versionId, type),
  });

  useEffect(() => {
    if (!isSuccess) return;
    setItems(saved?.items ?? []);
    setHydrated(true);
  }, [saved, isSuccess]);

  // Строка без месяца в расчёт не идёт, поэтому и сохранять её незачем.
  const payload: InputPurchases = {
    items: items.filter(p => p.month >= 1 && p.month <= duration),
  };

  const save = useCallback(
    (d: InputPurchases) => budgetsApi.saveInput(versionId, type, d),
    [versionId, type],
  );

  useAutosave({ data: payload, ready: hydrated, save, enabled: !readonly });

  const months = Array.from({ length: duration }, (_, i) => monthLabel(startDate, i));

  // Ровно то, что считает calcPurchases: суммы разложены по месяцам строк.
  const monthTotals = Array(duration).fill(0) as number[];
  items.forEach(p => {
    if (p.month >= 1 && p.month <= duration) {
      monthTotals[p.month - 1] += p.price * p.count;
    }
  });
  const grandTotal = monthTotals.reduce((s, v) => s + v, 0);

  return (
    <Card
      title={titleWithHint(title, hint)}
      size="small"
      extra={<Text type="secondary" style={{ fontSize: 12 }}>
        Итого {fmtNum(grandTotal)} ₽
      </Text>}
    >
      <PurchaseTable
        items={items}
        onChange={setItems}
        months={months}
        readonly={readonly}
        nameLabel={nameLabel}
        namePlaceholder={namePlaceholder}
        monthColLabel={monthColLabel}
        priceLabel={priceLabel}
        countLabel={countLabel}
        addLabel={addLabel}
      />

      {/* Пока строк нет, итожить нечего — таблица из прочерков только
          мешает (правило пустых состояний). */}
      {items.length > 0 && (
        <div style={{ marginTop: 16 }}>
          <MonthTotals months={months} values={monthTotals} label="Итого по месяцам" />
        </div>
      )}
    </Card>
  );
}
