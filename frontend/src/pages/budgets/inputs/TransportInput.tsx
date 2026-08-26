import { useCallback, useEffect, useState } from 'react';
import {
  Card, Input, InputNumber, Select, Button, Switch, Typography, Alert, Space,
} from 'antd';
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import { useQuery } from '@tanstack/react-query';
import { budgetsApi } from '../../../api';
import type { InputTransport, CarPurchase, RentedItem } from '../../../types';
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

/** Старый формат листа 4.3 — готовые суммы по месяцам. */
interface LegacyCost { monthly_amounts?: number[] }

const table: React.CSSProperties = {
  width: '100%',
  tableLayout: 'fixed',
  borderCollapse: 'collapse',
};

const NAME_COL = 200;
const TOTAL_COL = 110;
const DEL_COL = 40;

function emptyPurchase(): CarPurchase {
  return { name: '', month: 0, count: 1, price: 0 };
}

function emptyRental(duration: number): RentedItem {
  return { name: '', price: 0, counts: Array(duration).fill(0) };
}

/**
 * Приводит массив количеств к длине проекта: короткий добивается нулями,
 * длинный обрезается. Нужно после смены длительности проекта у уже
 * сохранённой версии — бэкенд поступает так же.
 */
function padCounts(counts: number[] | undefined, duration: number): number[] {
  const out = Array(duration).fill(0);
  (counts ?? []).slice(0, duration).forEach((v, i) => { out[i] = v || 0; });
  return out;
}

/** Итог строки аренды: цена × количество, просуммированное по месяцам. */
function rentalTotal(r: RentedItem, duration: number): number {
  return padCounts(r.counts, duration).reduce((s, c) => s + r.price * c, 0);
}

export default function TransportInput({ versionId, duration, startDate, readonly }: Props) {
  const [purchases, setPurchases] = useState<CarPurchase[]>([]);
  const [cars, setCars] = useState<RentedItem[]>([]);
  const [garages, setGarages] = useState<RentedItem[]>([]);
  const [hydrated, setHydrated] = useState(false);
  // «Единое значение на все месяцы» — только режим ввода, на сервер не идёт.
  const [carsUniform, setCarsUniform] = useState<boolean[]>([]);
  const [garagesUniform, setGaragesUniform] = useState<boolean[]>([]);

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
  const monthOptions = months.map((label, i) => ({ value: i + 1, label }));

  function patchRental(
    set: React.Dispatch<React.SetStateAction<RentedItem[]>>,
    idx: number,
    patch: Partial<RentedItem>,
  ) {
    set(prev => prev.map((r, i) => (i === idx ? { ...r, ...patch } : r)));
  }

  /** Количество в одном месяце; в режиме «единое» — сразу во всех. */
  function setCount(
    set: React.Dispatch<React.SetStateAction<RentedItem[]>>,
    isUniform: boolean,
    idx: number,
    monthIdx: number,
    value: number,
  ) {
    set(prev => prev.map((r, i) => {
      if (i !== idx) return r;
      const counts = isUniform
        ? Array(duration).fill(value)
        : padCounts(r.counts, duration).map((v, m) => (m === monthIdx ? value : v));
      return { ...r, counts };
    }));
  }

  /**
   * Переключение «единое значение»: при включении первое количество
   * размножается на все месяцы проекта.
   */
  function setUniform(
    set: React.Dispatch<React.SetStateAction<RentedItem[]>>,
    setFlags: React.Dispatch<React.SetStateAction<boolean[]>>,
    idx: number,
    on: boolean,
  ) {
    setFlags(prev => {
      const next = [...prev];
      next[idx] = on;
      return next;
    });
    if (!on) return;
    set(prev => prev.map((r, i) => (
      i === idx ? { ...r, counts: Array(duration).fill(padCounts(r.counts, duration)[0]) } : r
    )));
  }

  /**
   * Таблица аренды: строка = вид авто/гаража, колонки = месяцы с
   * количеством. Ширина фиксированная на всю белую область — тот же
   * MonthGrid, что в аренде квартир.
   */
  function renderRentalTable(
    items: RentedItem[],
    set: React.Dispatch<React.SetStateAction<RentedItem[]>>,
    flags: boolean[],
    setFlags: React.Dispatch<React.SetStateAction<boolean[]>>,
    namePlaceholder: string,
  ) {
    if (items.length === 0) {
      return (
        <Text type="secondary" style={{ fontSize: 12 }}>Строк нет</Text>
      );
    }
    return (
      <MonthGrid
        months={months}
        labelWidth={NAME_COL}
        trailingWidth={TOTAL_COL}
        head={(
          <thead>
            <tr>
              <th style={{ ...monthGridHeadCell, textAlign: 'left', color: '#333', fontWeight: 500 }}>
                Вид / цена за ед. в месяц, ₽
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
          {items.map((r, idx) => {
            const counts = padCounts(r.counts, duration);
            const isUniform = !!flags[idx];
            return (
              <tr key={idx} style={{ borderTop: '1px solid #f0f0f0' }}>
                <td style={{ ...monthGridCell, textAlign: 'left', padding: '4px' }}>
                  {readonly ? (
                    <div style={{ fontSize: 12 }}>
                      <div>{r.name || '—'}</div>
                      <div style={{ color: '#888' }}>{r.price.toLocaleString('ru-RU')} ₽</div>
                    </div>
                  ) : (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                      <Input
                        size="small"
                        value={r.name}
                        placeholder={namePlaceholder}
                        onChange={e => patchRental(set, idx, { name: e.target.value })}
                      />
                      <div style={{ display: 'flex', gap: 4, alignItems: 'center' }}>
                        <InputNumber
                          size="small"
                          style={{ width: '100%' }}
                          min={0}
                          value={r.price || null}
                          placeholder="цена"
                          onChange={v => patchRental(set, idx, { price: v ?? 0 })}
                          formatter={thousandFormatter}
                          parser={thousandParser}
                        />
                        <Button
                          size="small"
                          type="text"
                          danger
                          icon={<DeleteOutlined />}
                          onClick={() => set(prev => prev.filter((_, i) => i !== idx))}
                        />
                      </div>
                    </div>
                  )}
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
                        // Пусто = 0: строка есть, но в этом месяце не арендуем.
                        value={v || null}
                        placeholder="0"
                        // В режиме «единое значение» правим только первую ячейку
                        disabled={isUniform && i > 0}
                        onChange={val => setCount(set, isUniform, idx, i, val ?? 0)}
                      />
                    )}
                  </td>
                ))}
                <td style={{ ...monthGridCell, textAlign: 'right', fontWeight: 500, fontSize: 12 }}>
                  <div>{rentalTotal(r, duration).toLocaleString('ru-RU')}</div>
                  {!readonly && (
                    <div style={{ display: 'flex', alignItems: 'center', gap: 4, justifyContent: 'flex-end' }}>
                      <Switch
                        size="small"
                        checked={isUniform}
                        onChange={c => setUniform(set, setFlags, idx, c)}
                      />
                      <span style={{ fontSize: 10, color: '#888', fontWeight: 400 }}>единое</span>
                    </div>
                  )}
                </td>
              </tr>
            );
          })}
        </tbody>
      </MonthGrid>
    );
  }

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
        <table style={table}>
          <colgroup>
            <col />
            <col style={{ width: 180 }} />
            <col style={{ width: 160 }} />
            <col style={{ width: 70 }} />
            <col style={{ width: TOTAL_COL }} />
            {!readonly && <col style={{ width: DEL_COL }} />}
          </colgroup>
          <thead>
            <tr>
              <th style={{ ...monthGridHeadCell, textAlign: 'left', color: '#333', fontWeight: 500 }}>
                Описание авто
              </th>
              <th style={{ ...monthGridHeadCell, color: '#333', fontWeight: 500 }}>
                Месяц покупки
              </th>
              <th style={{ ...monthGridHeadCell, color: '#333', fontWeight: 500 }}>
                Цена за ед., ₽
              </th>
              <th style={{ ...monthGridHeadCell, color: '#333', fontWeight: 500 }}>
                Кол-во
              </th>
              <th style={{ ...monthGridHeadCell, textAlign: 'right', color: '#333', fontWeight: 500 }}>
                Итого
              </th>
              {!readonly && <th style={monthGridHeadCell} />}
            </tr>
          </thead>
          <tbody>
            {purchases.length === 0 && (
              <tr>
                <td
                  colSpan={5 + (readonly ? 0 : 1)}
                  style={{ ...monthGridCell, color: '#999', fontSize: 12, padding: '10px 4px' }}
                >
                  Покупок нет
                </td>
              </tr>
            )}
            {purchases.map((p, idx) => (
              <tr key={idx} style={{ borderTop: '1px solid #f0f0f0' }}>
                <td style={{ ...monthGridCell, textAlign: 'left' }}>
                  {readonly ? (
                    <Text style={{ fontSize: 12 }}>{p.name || '—'}</Text>
                  ) : (
                    <Input
                      size="small"
                      value={p.name}
                      placeholder="например, Газель NEXT"
                      onChange={e => setPurchases(prev => prev.map(
                        (r, i) => (i === idx ? { ...r, name: e.target.value } : r),
                      ))}
                    />
                  )}
                </td>
                <td style={monthGridCell}>
                  {readonly ? (
                    <Text style={{ fontSize: 12 }}>
                      {p.month >= 1 ? monthLabel(startDate, p.month - 1) : '—'}
                    </Text>
                  ) : (
                    <Select
                      size="small"
                      style={{ width: '100%' }}
                      // Список ограничен месяцами проекта — выбрать месяц
                      // за его пределами нельзя.
                      options={monthOptions}
                      value={p.month >= 1 && p.month <= duration ? p.month : undefined}
                      placeholder="выберите месяц"
                      status={p.month >= 1 ? undefined : 'error'}
                      onChange={v => setPurchases(prev => prev.map(
                        (r, i) => (i === idx ? { ...r, month: v ?? 0 } : r),
                      ))}
                    />
                  )}
                </td>
                <td style={monthGridCell}>
                  {readonly ? (
                    <Text style={{ fontSize: 12 }}>{p.price.toLocaleString('ru-RU')}</Text>
                  ) : (
                    <InputNumber
                      size="small"
                      style={{ width: '100%' }}
                      min={0}
                      value={p.price || null}
                      onChange={v => setPurchases(prev => prev.map(
                        (r, i) => (i === idx ? { ...r, price: v ?? 0 } : r),
                      ))}
                      formatter={thousandFormatter}
                      parser={thousandParser}
                    />
                  )}
                </td>
                <td style={monthGridCell}>
                  {readonly ? (
                    <Text style={{ fontSize: 12 }}>{p.count}</Text>
                  ) : (
                    <InputNumber
                      size="small"
                      style={{ width: '100%' }}
                      min={0}
                      value={p.count}
                      onChange={v => setPurchases(prev => prev.map(
                        (r, i) => (i === idx ? { ...r, count: v ?? 0 } : r),
                      ))}
                    />
                  )}
                </td>
                <td style={{ ...monthGridCell, textAlign: 'right', fontWeight: 500, fontSize: 12 }}>
                  {(p.price * p.count).toLocaleString('ru-RU')}
                </td>
                {!readonly && (
                  <td style={monthGridCell}>
                    <Button
                      size="small"
                      type="text"
                      danger
                      icon={<DeleteOutlined />}
                      onClick={() => setPurchases(prev => prev.filter((_, i) => i !== idx))}
                    />
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
        {!readonly && (
          <Button
            size="small"
            type="dashed"
            icon={<PlusOutlined />}
            style={{ marginTop: 8 }}
            onClick={() => setPurchases(prev => [...prev, emptyPurchase()])}
          >
            Добавить покупку
          </Button>
        )}
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
        {renderRentalTable(cars, setCars, carsUniform, setCarsUniform, 'например, Газель')}
        {!readonly && (
          <Button
            size="small"
            type="dashed"
            icon={<PlusOutlined />}
            style={{ marginTop: 8 }}
            onClick={() => setCars(prev => [...prev, emptyRental(duration)])}
          >
            Добавить авто
          </Button>
        )}
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
        {renderRentalTable(garages, setGarages, garagesUniform, setGaragesUniform, 'например, Гараж №1')}
        {!readonly && (
          <Button
            size="small"
            type="dashed"
            icon={<PlusOutlined />}
            style={{ marginTop: 8 }}
            onClick={() => setGarages(prev => [...prev, emptyRental(duration)])}
          >
            Добавить гараж
          </Button>
        )}
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
