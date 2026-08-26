import { useCallback, useEffect, useState } from 'react';
import { Card, InputNumber, Typography, Alert } from 'antd';
import { useQuery } from '@tanstack/react-query';
import { budgetsApi } from '../../../api';
import type { InputWagonciks, MonthlyQty } from '../../../types';
import { purchaseAllowedMonths } from '../../../types';
import { monthLabel, thousandFormatter, thousandParser } from '../../../utils/fmt';
import MonthGrid, { monthGridCell, monthGridHeadCell } from '../../../components/MonthGrid';
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

function emptyQty(duration: number): MonthlyQty {
  return { price: 0, counts: Array(duration).fill(0) };
}

/** Приводит массив количеств к длине проекта; бэкенд поступает так же. */
function padCounts(counts: number[] | undefined, duration: number): number[] {
  const out = Array(duration).fill(0);
  (counts ?? []).slice(0, duration).forEach((v, i) => { out[i] = v || 0; });
  return out;
}

export default function WagonciksInput({ versionId, duration, startDate, readonly }: Props) {
  const [rental, setRental] = useState<MonthlyQty>(emptyQty(duration));
  const [purchase, setPurchase] = useState<MonthlyQty>(emptyQty(duration));
  const [hydrated, setHydrated] = useState(false);

  const { data: saved, isSuccess } = useQuery({
    queryKey: ['budget-input', versionId, 'wagonciks'],
    queryFn: () => budgetsApi.getInput<InputWagonciks>(versionId, 'wagonciks'),
  });

  useEffect(() => {
    if (!isSuccess) return;
    const pick = (q: MonthlyQty | undefined): MonthlyQty => ({
      price: q?.price ?? 0,
      counts: padCounts(q?.counts, duration),
    });
    setRental(pick(saved?.rental));
    setPurchase(pick(saved?.purchase));
    setHydrated(true);
  }, [saved, isSuccess, duration]);

  // Покупка недоступна в последние два месяца проекта — правило владельца
  // 2026-08-26. Количества за пределами allowed на сервер не отправляем:
  // расчёт их всё равно обнулит, а в сохранённых данных они бы сбивали с толку.
  const allowed = purchaseAllowedMonths(duration);

  const payload: InputWagonciks = {
    rental: { price: rental.price, counts: padCounts(rental.counts, duration) },
    purchase: {
      price: purchase.price,
      counts: padCounts(purchase.counts, duration).map((v, i) => (i < allowed ? v : 0)),
    },
  };

  const save = useCallback(
    (d: InputWagonciks) => budgetsApi.saveInput(versionId, 'wagonciks', d),
    [versionId],
  );

  useAutosave({ data: payload, ready: hydrated, save, enabled: !readonly });

  const months = Array.from({ length: duration }, (_, i) => monthLabel(startDate, i));

  function setCount(
    set: React.Dispatch<React.SetStateAction<MonthlyQty>>,
    monthIdx: number,
    value: number,
  ) {
    set(prev => ({
      ...prev,
      counts: padCounts(prev.counts, duration).map((v, i) => (i === monthIdx ? value : v)),
    }));
  }

  /**
   * Одна строка «Количество» на сетке месяцев. blockedFrom — индекс, начиная
   * с которого ячейки заблокированы (для аренды не задаётся).
   */
  function renderQtyGrid(
    qty: MonthlyQty,
    set: React.Dispatch<React.SetStateAction<MonthlyQty>>,
    blockedFrom?: number,
  ) {
    const counts = padCounts(qty.counts, duration);
    const total = counts.reduce(
      (s, c, i) => s + (blockedFrom !== undefined && i >= blockedFrom ? 0 : qty.price * c),
      0,
    );
    return (
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
            {counts.map((v, i) => {
              const blocked = blockedFrom !== undefined && i >= blockedFrom;
              return (
                <td key={i} style={monthGridCell}>
                  {blocked ? (
                    // Последние месяцы проекта: покупка недоступна.
                    <Text type="secondary">—</Text>
                  ) : readonly ? (
                    <Text style={{ fontSize: 12 }}>{v || '—'}</Text>
                  ) : (
                    <InputNumber
                      size="small"
                      style={{ width: '100%' }}
                      min={0}
                      precision={0}
                      // Пусто = 0: в этом месяце ничего нет.
                      value={v || null}
                      placeholder="0"
                      onChange={val => setCount(set, i, val ?? 0)}
                    />
                  )}
                </td>
              );
            })}
            <td style={{ ...monthGridCell, textAlign: 'right', fontWeight: 500, fontSize: 12 }}>
              {total.toLocaleString('ru-RU')} ₽
            </td>
          </tr>
        </tbody>
      </MonthGrid>
    );
  }

  function renderPrice(
    qty: MonthlyQty,
    set: React.Dispatch<React.SetStateAction<MonthlyQty>>,
    label: string,
  ) {
    return (
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 12 }}>
        <Text style={{ fontSize: 12 }}>{label}</Text>
        {readonly ? (
          <Text strong>{qty.price.toLocaleString('ru-RU')} ₽</Text>
        ) : (
          <InputNumber
            size="small"
            style={{ width: 180 }}
            min={0}
            value={qty.price || null}
            placeholder="0"
            onChange={v => set(prev => ({ ...prev, price: v ?? 0 }))}
            formatter={thousandFormatter}
            parser={thousandParser}
            addonAfter="₽"
          />
        )}
      </div>
    );
  }

  const rentalTotal = padCounts(rental.counts, duration)
    .reduce((s, c) => s + rental.price * c, 0);
  const purchaseTotal = padCounts(purchase.counts, duration)
    .reduce((s, c, i) => s + (i < allowed ? purchase.price * c : 0), 0);

  return (
    <div>
      <Card
        title="Аренда вагончиков"
        size="small"
        style={{ marginBottom: 16 }}
        extra={<Text type="secondary" style={{ fontSize: 12 }}>
          Итого {rentalTotal.toLocaleString('ru-RU')} ₽
        </Text>}
      >
        {renderPrice(rental, setRental, 'Цена аренды одного вагончика в месяц:')}
        {renderQtyGrid(rental, setRental)}
      </Card>

      <Card
        title="Покупка вагончиков"
        size="small"
        extra={<Text type="secondary" style={{ fontSize: 12 }}>
          Итого {purchaseTotal.toLocaleString('ru-RU')} ₽
        </Text>}
      >
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message="Покупка недоступна в последние 2 месяца проекта"
        />
        {renderPrice(purchase, setPurchase, 'Цена одного вагончика:')}
        {renderQtyGrid(purchase, setPurchase, allowed)}
      </Card>

      <Text type="secondary" style={{ display: 'block', marginTop: 12, fontSize: 12 }}>
        Аренда и покупка складываются в одну строку бюджета «Обустройство
        строительной площадки (вагончики)» —{' '}
        {(rentalTotal + purchaseTotal).toLocaleString('ru-RU')} ₽ за проект.
      </Text>
    </div>
  );
}
