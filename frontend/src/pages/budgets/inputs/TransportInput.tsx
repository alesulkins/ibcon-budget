import { useCallback, useEffect, useState } from 'react';
import { Card, Typography, Alert, Space } from 'antd';
import { useQuery } from '@tanstack/react-query';
import { budgetsApi } from '../../../api';
import type { InputTransport, ItemPurchase, RentedItem } from '../../../types';
import { monthLabel, thousandFormatter, thousandParser } from '../../../utils/fmt';
import PurchaseTable from '../../../components/PurchaseTable';
import RentalGrid, { padCounts, rentalTotal } from '../../../components/RentalGrid';
import { useAutosave } from '../../../hooks/useAutosave';

const { Text } = Typography;

interface Props {
  versionId: number;
  duration: number;
  startDate: string;
  readonly?: boolean;
}

/** Старый формат листа 4.3 — готовые суммы по месяцам. */
interface LegacyCost { monthly_amounts?: number[] }


export default function TransportInput({ versionId, duration, startDate, readonly }: Props) {
  const [purchases, setPurchases] = useState<ItemPurchase[]>([]);
  const [cars, setCars] = useState<RentedItem[]>([]);
  const [garages, setGarages] = useState<RentedItem[]>([]);
  const [hydrated, setHydrated] = useState(false);

  const { data: saved, isSuccess } = useQuery({
    queryKey: ['budget-input', versionId, 'transport'],
    queryFn: () => budgetsApi.getInput<InputTransport>(versionId, 'transport'),
  });

  // Версии, сохранённые до перехода 4.3 на расчёт по формуле, продолжают
  // считаться по старым суммам — предупреждаем, чтобы пустые таблицы не
  // выглядели противоречием с ненулевым результатом.
  const { data: legacyRental } = useQuery({
    queryKey: ['budget-input', versionId, 'transport_rental'],
    queryFn: () => budgetsApi.getInput<LegacyCost>(versionId, 'transport_rental'),
  });
  const { data: legacyGarage } = useQuery({
    queryKey: ['budget-input', versionId, 'garage_rent'],
    queryFn: () => budgetsApi.getInput<LegacyCost>(versionId, 'garage_rent'),
  });

  useEffect(() => {
    if (!isSuccess) return;
    const pick = (items: RentedItem[] | undefined) =>
      (items ?? []).map(r => ({ ...r, counts: padCounts(r.counts, duration) }));
    setPurchases(saved?.car_purchases ?? []);
    setCars(pick(saved?.car_rentals));
    setGarages(pick(saved?.garage_rentals));
    setHydrated(true);
  }, [saved, isSuccess, duration]);

  const legacySum = (d: LegacyCost | null | undefined) =>
    (d?.monthly_amounts ?? []).reduce((s, v) => s + (v || 0), 0);
  const hasLegacy =
    !saved && (legacySum(legacyRental) > 0 || legacySum(legacyGarage) > 0);

  // Строка покупки без месяца игнорируется и при расчёте, и при сохранении:
  // в форме такая строка тоже не попадала ни в один месяц (SUMPRODUCT не
  // находил колонку), только там это происходило молча.
  const keepRental = (r: RentedItem) =>
    r.price > 0 || r.name !== '' || padCounts(r.counts, duration).some(c => c > 0);

  const payload: InputTransport = {
    car_purchases: purchases.filter(p => p.month >= 1 && p.month <= duration),
    car_rentals: cars.filter(keepRental)
      .map(r => ({ ...r, counts: padCounts(r.counts, duration) })),
    garage_rentals: garages.filter(keepRental)
      .map(r => ({ ...r, counts: padCounts(r.counts, duration) })),
  };

  const save = useCallback(
    (d: InputTransport) => budgetsApi.saveInput(versionId, 'transport', d),
    [versionId],
  );

  useAutosave({ data: payload, ready: hydrated, save, enabled: !readonly });

  const months = Array.from({ length: duration }, (_, i) => monthLabel(startDate, i));


  const purchasesTotal = purchases
    .filter(p => p.month >= 1 && p.month <= duration)
    .reduce((s, p) => s + p.price * p.count, 0);
  const carsTotal = cars.reduce((s, r) => s + rentalTotal(r, duration), 0);
  const garagesTotal = garages.reduce((s, r) => s + rentalTotal(r, duration), 0);

  return (
    <div>
      {hasLegacy && (
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 16 }}
          message="По этому шагу сохранены данные в старом формате"
          description={
            'Раньше транспорт и гараж вводились готовыми суммами по месяцам. '
            + 'Эти суммы пока участвуют в расчёте. Заполните таблицы ниже — '
            + 'после первого сохранения расчёт перейдёт на формулы листа 4.3, '
            + 'а старые суммы перестанут учитываться.'
          }
        />
      )}

      <Card
        title="Покупка авто"
        size="small"
        style={{ marginBottom: 16 }}
        extra={<Text type="secondary" style={{ fontSize: 12 }}>
          Итого {purchasesTotal.toLocaleString('ru-RU')} ₽
        </Text>}
      >
        <Text type="secondary" style={{ display: 'block', marginBottom: 8, fontSize: 12 }}>
          Разовая покупка учитывается целиком в месяц приобретения. Месяц —
          обязателен: строка без месяца в расчёт не попадёт и не сохранится.
          Итог строки — цена × количество.
        </Text>
        <PurchaseTable
          items={purchases}
          onChange={setPurchases}
          months={months}
          readonly={readonly}
          namePlaceholder="например, Газель NEXT"
          addLabel="Добавить покупку"
          emptyLabel="Покупок нет"
        />
      </Card>

      <Card
        title="Аренда авто"
        size="small"
        style={{ marginBottom: 16 }}
        extra={<Text type="secondary" style={{ fontSize: 12 }}>
          Итого {carsTotal.toLocaleString('ru-RU')} ₽
        </Text>}
      >
        <Text type="secondary" style={{ display: 'block', marginBottom: 8, fontSize: 12 }}>
          Одна строка — один вид авто со своей ценой. В каждом месяце укажите
          количество арендованных единиц; пусто или 0 — в этом месяце не
          арендуем. Расход месяца — цена × количество, просуммированное по
          всем видам.
        </Text>
        <RentalGrid
          items={cars}
          onChange={setCars}
          months={months}
          readonly={readonly}
          headLabel="Вид / цена за ед. в месяц, ₽"
          namePlaceholder="например, Газель"
          addLabel="Добавить авто"
          emptyLabel="Строк нет"
        />
      </Card>

      <Card
        title="Аренда гаража"
        size="small"
        extra={<Text type="secondary" style={{ fontSize: 12 }}>
          Итого {garagesTotal.toLocaleString('ru-RU')} ₽
        </Text>}
      >
        <Text type="secondary" style={{ display: 'block', marginBottom: 8, fontSize: 12 }}>
          Отдельная строка расходов бюджета — «Аренда гаража». Считается так
          же: цена × количество в каждом месяце.
        </Text>
        <RentalGrid
          items={garages}
          onChange={setGarages}
          months={months}
          readonly={readonly}
          headLabel="Гараж / цена за ед. в месяц, ₽"
          namePlaceholder="например, Гараж №1"
          addLabel="Добавить гараж"
          emptyLabel="Строк нет"
        />
      </Card>

      <Space style={{ marginTop: 12 }}>
        <Text type="secondary" style={{ fontSize: 12 }}>
          Покупка и аренда авто складываются в одну строку бюджета «Аренда
          транспорта» ({(purchasesTotal + carsTotal).toLocaleString('ru-RU')} ₽);
          гараж идёт отдельной строкой ({garagesTotal.toLocaleString('ru-RU')} ₽).
        </Text>
      </Space>
    </div>
  );
}
