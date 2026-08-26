import { useCallback, useEffect, useState } from 'react';
import { Card, InputNumber, Typography, Alert } from 'antd';
import { useQuery } from '@tanstack/react-query';
import { budgetsApi } from '../../../api';
import type { InputWagonciks, MonthlyQty, ItemPurchase } from '../../../types';
import { purchaseAllowedMonths } from '../../../types';
import { monthLabel, thousandFormatter, thousandParser } from '../../../utils/fmt';
import MonthGrid, { monthGridCell, monthGridHeadCell } from '../../../components/MonthGrid';
import PurchaseTable from '../../../components/PurchaseTable';
import { useAutosave } from '../../../hooks/useAutosave';

const { Text } = Typography;

interface Props {
  versionId: number;
  duration: number;
  startDate: string;
  readonly?: boolean;
}

const LABEL_COL = 160;
const TOTAL_COL = 120;

/** Приводит массив количеств к длине проекта; бэкенд поступает так же. */
function padCounts(counts: number[] | undefined, duration: number): number[] {
  const out = Array(duration).fill(0);
  (counts ?? []).slice(0, duration).forEach((v, i) => { out[i] = v || 0; });
  return out;
}

export default function WagonciksInput({ versionId, duration, startDate, readonly }: Props) {
  const [rental, setRental] = useState<MonthlyQty>({ price: 0, counts: Array(duration).fill(0) });
  const [purchases, setPurchases] = useState<ItemPurchase[]>([]);
  const [hydrated, setHydrated] = useState(false);

  const { data: saved, isSuccess } = useQuery({
    queryKey: ['budget-input', versionId, 'wagonciks'],
    queryFn: () => budgetsApi.getInput<InputWagonciks>(versionId, 'wagonciks'),
  });

  useEffect(() => {
    if (!isSuccess) return;
    setRental({
      price: saved?.rental?.price ?? 0,
      counts: padCounts(saved?.rental?.counts, duration),
    });
    setPurchases(saved?.purchases ?? []);
    setHydrated(true);
  }, [saved, isSuccess, duration]);

  // Покупка вагончиков невозможна в последние два месяца проекта — правило
  // владельца 2026-08-26. Именно оно закодировано границей расщепления
  // формулы в 4.4!строка 6 (SUMPRODUCT только в первых колонках).
  const allowed = purchaseAllowedMonths(duration);

  // Строка покупки без месяца игнорируется и при расчёте, и при сохранении.
  const payload: InputWagonciks = {
    rental: { price: rental.price, counts: padCounts(rental.counts, duration) },
    purchases: purchases.filter(p => p.month >= 1 && p.month <= allowed),
  };

  const save = useCallback(
    (d: InputWagonciks) => budgetsApi.saveInput(versionId, 'wagonciks', d),
    [versionId],
  );

  useAutosave({ data: payload, ready: hydrated, save, enabled: !readonly });

  const months = Array.from({ length: duration }, (_, i) => monthLabel(startDate, i));

  function setCount(monthIdx: number, value: number) {
    setRental(prev => ({
      ...prev,
      counts: padCounts(prev.counts, duration).map((v, i) => (i === monthIdx ? value : v)),
    }));
  }

  const counts = padCounts(rental.counts, duration);
  const rentalTotal = counts.reduce((s, c) => s + rental.price * c, 0);
  const purchasesTotal = purchases
    .filter(p => p.month >= 1 && p.month <= allowed)
    .reduce((s, p) => s + p.price * p.count, 0);

  return (
    <div>
      <Card
        title="Покупка вагончиков"
        size="small"
        style={{ marginBottom: 16 }}
        extra={<Text type="secondary" style={{ fontSize: 12 }}>
          Итого {purchasesTotal.toLocaleString('ru-RU')} ₽
        </Text>}
      >
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message="Покупка недоступна в последние 2 месяца проекта"
          description={allowed > 0
            ? `Выбрать можно месяцы с 1-го по ${allowed}-й.`
            : 'При такой длительности проекта покупка недоступна ни в одном месяце.'}
        />
        <Text type="secondary" style={{ display: 'block', marginBottom: 8, fontSize: 12 }}>
          Разовая покупка учитывается целиком в месяц приобретения. Месяц —
          обязателен: строка без месяца в расчёт не попадёт и не сохранится.
          Итог строки — цена × количество.
        </Text>
        <PurchaseTable
          items={purchases}
          onChange={setPurchases}
          months={months}
          allowedMonths={allowed}
          readonly={readonly}
          namePlaceholder="например, Бытовка 6×2,4"
          addLabel="Добавить покупку"
          emptyLabel="Покупок нет"
        />
      </Card>

      <Card
        title="Аренда вагончиков"
        size="small"
        extra={<Text type="secondary" style={{ fontSize: 12 }}>
          Итого {rentalTotal.toLocaleString('ru-RU')} ₽
        </Text>}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 12 }}>
          <Text style={{ fontSize: 12 }}>Цена аренды одного вагончика в месяц:</Text>
          {readonly ? (
            <Text strong>{rental.price.toLocaleString('ru-RU')} ₽</Text>
          ) : (
            <InputNumber
              size="small"
              style={{ width: 180 }}
              min={0}
              value={rental.price || null}
              placeholder="0"
              onChange={v => setRental(prev => ({ ...prev, price: v ?? 0 }))}
              formatter={thousandFormatter}
              parser={thousandParser}
              addonAfter="₽"
            />
          )}
        </div>

        <MonthGrid
          months={months}
          labelWidth={LABEL_COL}
          trailingWidth={TOTAL_COL}
          head={(
            <thead>
              <tr>
                <th style={{ ...monthGridHeadCell, textAlign: 'left', color: '#333', fontWeight: 500 }}>
                  Количество, шт
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
            <tr>
              <td style={{ ...monthGridCell, textAlign: 'left', fontSize: 12, color: '#888' }}>
                по месяцам
              </td>
              {counts.map((v, i) => (
                <td key={i} style={monthGridCell}>
                  {readonly ? (
                    <Text style={{ fontSize: 12 }}>{v || '—'}</Text>
                  ) : (
                    <InputNumber
                      size="small"
                      style={{ width: '100%' }}
                      min={0}
                      precision={0}
                      // Пусто = 0: в этом месяце вагончики не арендуем.
                      value={v || null}
                      placeholder="0"
                      onChange={val => setCount(i, val ?? 0)}
                    />
                  )}
                </td>
              ))}
              <td style={{ ...monthGridCell, textAlign: 'right', fontWeight: 500, fontSize: 12 }}>
                {rentalTotal.toLocaleString('ru-RU')} ₽
              </td>
            </tr>
          </tbody>
        </MonthGrid>
      </Card>

      <Text type="secondary" style={{ display: 'block', marginTop: 12, fontSize: 12 }}>
        Покупка и аренда складываются в одну строку бюджета «Обустройство
        строительной площадки (вагончики)» —{' '}
        {(rentalTotal + purchasesTotal).toLocaleString('ru-RU')} ₽ за проект.
      </Text>
    </div>
  );
}
