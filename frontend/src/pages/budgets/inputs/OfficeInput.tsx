import { useCallback, useEffect, useState } from 'react';
import { Card, InputNumber, Typography } from 'antd';
import { useQuery } from '@tanstack/react-query';
import { budgetsApi } from '../../../api';
import type { InputOffice, RentedItem } from '../../../types';
import { monthLabel, thousandFormatter, thousandParser } from '../../../utils/fmt';
import RentalGrid, { padCounts, rentalTotal } from '../../../components/RentalGrid';
import { useAutosave } from '../../../hooks/useAutosave';

const { Text } = Typography;

interface Props {
  versionId: number;
  duration: number;
  startDate: string;
  /** Название исполнителя проекта — от него зависит gross-up на НДФЛ. */
  executor: string;
  readonly?: boolean;
}

const EXECUTOR_KG = 'Айбикон Киргизия';

export default function OfficeInput({
  versionId, duration, startDate, executor, readonly,
}: Props) {
  const [offices, setOffices] = useState<RentedItem[]>([]);
  const [cleaningPrice, setCleaningPrice] = useState(0);
  const [hydrated, setHydrated] = useState(false);

  const { data: saved, isSuccess } = useQuery({
    queryKey: ['budget-input', versionId, 'office'],
    queryFn: () => budgetsApi.getInput<InputOffice>(versionId, 'office'),
  });

  useEffect(() => {
    if (!isSuccess) return;
    setOffices((saved?.offices ?? []).map(o => ({ ...o, counts: padCounts(o.counts, duration) })));
    setCleaningPrice(saved?.cleaning_price ?? 0);
    setHydrated(true);
  }, [saved, isSuccess, duration]);

  const keep = (o: RentedItem) =>
    o.price > 0 || o.name !== '' || padCounts(o.counts, duration).some(c => c > 0);

  const payload: InputOffice = {
    offices: offices.filter(keep).map(o => ({ ...o, counts: padCounts(o.counts, duration) })),
    cleaning_price: cleaningPrice,
  };

  const save = useCallback(
    (d: InputOffice) => budgetsApi.saveInput(versionId, 'office', d),
    [versionId],
  );

  useAutosave({ data: payload, ready: hydrated, save, enabled: !readonly });

  const months = Array.from({ length: duration }, (_, i) => monthLabel(startDate, i));

  // Аренда офиса делится на 0.87 — gross-up на НДФЛ 13% (4.5!C5).
  // У киргизского филиала деления нет.
  const isKG = executor?.trim().toLowerCase() === EXECUTOR_KG.toLowerCase();
  const GROSS_UP = 0.87;

  const officeCosts = offices.reduce((s, o) => s + rentalTotal(o, duration), 0);
  const rentTotal = isKG ? officeCosts : officeCosts / GROSS_UP;

  // Уборка идёт от количества офисов в месяце, а не от их стоимости.
  const monthlyCounts = Array.from({ length: duration }, (_, m) =>
    offices.reduce((s, o) => s + padCounts(o.counts, duration)[m], 0));
  const cleaningTotal = monthlyCounts.reduce((s, c) => s + c * cleaningPrice, 0);

  return (
    <div>
      <Card
        title="Аренда офиса"
        size="small"
        style={{ marginBottom: 16 }}
        extra={<Text type="secondary" style={{ fontSize: 12 }}>
          Итого {rentTotal.toLocaleString('ru-RU', { maximumFractionDigits: 2 })} ₽
        </Text>}
      >
        <Text type="secondary" style={{ display: 'block', marginBottom: 8, fontSize: 12 }}>
          Одна строка — один офис со своей ценой. В каждом месяце укажите
          количество арендованных офисов; пусто или 0 — в этом месяце офис не
          арендуем.
          {isKG
            ? ' Для киргизского филиала сумма идёт в бюджет как есть.'
            : ` В бюджет сумма попадает с надбавкой на НДФЛ: делится на ${GROSS_UP}`
              + ` (стоимость офисов ${officeCosts.toLocaleString('ru-RU')} ₽ →`
              + ` ${rentTotal.toLocaleString('ru-RU', { maximumFractionDigits: 2 })} ₽).`}
        </Text>
        <RentalGrid
          items={offices}
          onChange={setOffices}
          months={months}
          readonly={readonly}
          headLabel="Офис / цена за ед. в месяц, ₽"
          namePlaceholder="например, Офис на Ленина"
          addLabel="Добавить офис"
          emptyLabel="Офисов нет"
        />
      </Card>

      <Card
        title="Уборка офиса"
        size="small"
        extra={<Text type="secondary" style={{ fontSize: 12 }}>
          Итого {cleaningTotal.toLocaleString('ru-RU')} ₽
        </Text>}
      >
        <Text type="secondary" style={{ display: 'block', marginBottom: 8, fontSize: 12 }}>
          Отдельная строка расходов бюджета. Уборка начисляется каждый месяц
          на каждый арендованный офис, поэтому укажите стоимость уборки
          <b> одного офиса за месяц</b> — расход месяца считается сам:
          количество офисов × эта цена. Надбавка на НДФЛ к уборке не
          применяется.
        </Text>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <Text style={{ fontSize: 12 }}>Стоимость уборки одного офиса в месяц:</Text>
          {readonly ? (
            <Text strong>{cleaningPrice.toLocaleString('ru-RU')} ₽</Text>
          ) : (
            <InputNumber
              size="small"
              style={{ width: 180 }}
              min={0}
              value={cleaningPrice || null}
              placeholder="0"
              onChange={v => setCleaningPrice(v ?? 0)}
              formatter={thousandFormatter}
              parser={thousandParser}
              addonAfter="₽"
            />
          )}
        </div>
        <Text type="secondary" style={{ display: 'block', marginTop: 8, fontSize: 12 }}>
          Офисов по месяцам: {monthlyCounts.join(' · ') || '—'}
        </Text>
      </Card>
    </div>
  );
}
