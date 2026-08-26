import { useCallback, useEffect, useState } from 'react';
import {
  Card, Input, InputNumber, Select, Button, Checkbox, Typography, Alert, Space,
} from 'antd';
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import { useQuery } from '@tanstack/react-query';
import { budgetsApi } from '../../../api';
import type { InputTransport, CarPurchase, RentedItem } from '../../../types';
import { monthLabel, thousandFormatter, thousandParser } from '../../../utils/fmt';
import { monthGridCell, monthGridHeadCell } from '../../../components/MonthGrid';
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
const PRICE_COL = 130;
const TOTAL_COL = 110;
const DEL_COL = 40;

function emptyPurchase(): CarPurchase {
  return { name: '', month: 0, price: 0 };
}

function emptyRental(): RentedItem {
  return { name: '', price: 0, months: [] };
}

export default function TransportInput({ versionId, duration, startDate, readonly }: Props) {
  const [purchases, setPurchases] = useState<CarPurchase[]>([]);
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
    setPurchases(saved?.car_purchases ?? []);
    setCars(saved?.car_rentals ?? []);
    setGarages(saved?.garage_rentals ?? []);
    setHydrated(true);
  }, [saved, isSuccess]);

  const legacySum = (d: LegacyCost | null | undefined) =>
    (d?.monthly_amounts ?? []).reduce((s, v) => s + (v || 0), 0);
  const hasLegacy =
    !saved && (legacySum(legacyRental) > 0 || legacySum(legacyGarage) > 0);

  // Строка покупки без месяца игнорируется и при расчёте, и при сохранении:
  // в форме такая строка тоже не попадала ни в один месяц (SUMPRODUCT не
  // находил колонку), только там это происходило молча.
  const payload: InputTransport = {
    car_purchases: purchases.filter(p => p.month >= 1 && p.month <= duration),
    car_rentals: cars.filter(r => r.months.length > 0 || r.price > 0 || r.name !== ''),
    garage_rentals: garages.filter(r => r.months.length > 0 || r.price > 0 || r.name !== ''),
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

  function toggleMonth(
    set: React.Dispatch<React.SetStateAction<RentedItem[]>>,
    idx: number,
    monthNum: number,
    on: boolean,
  ) {
    set(prev => prev.map((r, i) => {
      if (i !== idx) return r;
      const months = on
        ? [...r.months, monthNum].sort((a, b) => a - b)
        : r.months.filter(m => m !== monthNum);
      return { ...r, months };
    }));
  }

  /** Таблица аренды: строка = предмет, колонки = месяцы с чекбоксами. */
  function renderRentalTable(
    items: RentedItem[],
    set: React.Dispatch<React.SetStateAction<RentedItem[]>>,
    namePlaceholder: string,
  ) {
    return (
      <table style={table}>
        <colgroup>
          <col style={{ width: NAME_COL }} />
          <col style={{ width: PRICE_COL }} />
          {months.map((_, i) => <col key={i} />)}
          <col style={{ width: TOTAL_COL }} />
          {!readonly && <col style={{ width: DEL_COL }} />}
        </colgroup>
        <thead>
          <tr>
            <th style={{ ...monthGridHeadCell, textAlign: 'left', color: '#333', fontWeight: 500 }}>
              Название
            </th>
            <th style={{ ...monthGridHeadCell, color: '#333', fontWeight: 500 }}>
              Цена в месяц, ₽
            </th>
            {months.map((m, i) => <th key={i} style={monthGridHeadCell}>{m}</th>)}
            <th style={{ ...monthGridHeadCell, textAlign: 'right', color: '#333', fontWeight: 500 }}>
              Итого
            </th>
            {!readonly && <th style={monthGridHeadCell} />}
          </tr>
        </thead>
        <tbody>
          {items.length === 0 && (
            <tr>
              <td
                colSpan={2 + duration + 1 + (readonly ? 0 : 1)}
                style={{ ...monthGridCell, color: '#999', fontSize: 12, padding: '10px 4px' }}
              >
                Строк нет
              </td>
            </tr>
          )}
          {items.map((r, idx) => (
            <tr key={idx} style={{ borderTop: '1px solid #f0f0f0' }}>
              <td style={{ ...monthGridCell, textAlign: 'left' }}>
                {readonly ? (
                  <Text style={{ fontSize: 12 }}>{r.name || '—'}</Text>
                ) : (
                  <Input
                    size="small"
                    value={r.name}
                    placeholder={namePlaceholder}
                    onChange={e => patchRental(set, idx, { name: e.target.value })}
                  />
                )}
              </td>
              <td style={monthGridCell}>
                {readonly ? (
                  <Text style={{ fontSize: 12 }}>{r.price.toLocaleString('ru-RU')}</Text>
                ) : (
                  <InputNumber
                    size="small"
                    style={{ width: '100%' }}
                    min={0}
                    value={r.price || null}
                    onChange={v => patchRental(set, idx, { price: v ?? 0 })}
                    formatter={thousandFormatter}
                    parser={thousandParser}
                  />
                )}
              </td>
              {months.map((_, i) => (
                <td key={i} style={monthGridCell}>
                  <Checkbox
                    checked={r.months.includes(i + 1)}
                    disabled={readonly}
                    onChange={e => toggleMonth(set, idx, i + 1, e.target.checked)}
                  />
                </td>
              ))}
              <td style={{ ...monthGridCell, textAlign: 'right', fontWeight: 500, fontSize: 12 }}>
                {(r.price * r.months.length).toLocaleString('ru-RU')}
              </td>
              {!readonly && (
                <td style={monthGridCell}>
                  <Button
                    size="small"
                    type="text"
                    danger
                    icon={<DeleteOutlined />}
                    onClick={() => set(prev => prev.filter((_, i) => i !== idx))}
                  />
                </td>
              )}
            </tr>
          ))}
        </tbody>
      </table>
    );
  }

  const purchasesTotal = purchases
    .filter(p => p.month >= 1 && p.month <= duration)
    .reduce((s, p) => s + (p.price || 0), 0);
  const carsTotal = cars.reduce((s, r) => s + r.price * r.months.length, 0);
  const garagesTotal = garages.reduce((s, r) => s + r.price * r.months.length, 0);

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
          Две одинаковые машины — две строки.
        </Text>
        <table style={table}>
          <colgroup>
            <col />
            <col style={{ width: 180 }} />
            <col style={{ width: 180 }} />
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
                Цена, ₽
              </th>
              {!readonly && <th style={monthGridHeadCell} />}
            </tr>
          </thead>
          <tbody>
            {purchases.length === 0 && (
              <tr>
                <td
                  colSpan={3 + (readonly ? 0 : 1)}
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
          Одна строка — один арендуемый автомобиль со своей ценой. Отметьте
          месяцы, в которых он арендуется. Две одинаковые машины — две строки.
        </Text>
        {renderRentalTable(cars, setCars, 'например, Газель')}
        {!readonly && (
          <Button
            size="small"
            type="dashed"
            icon={<PlusOutlined />}
            style={{ marginTop: 8 }}
            onClick={() => setCars(prev => [...prev, emptyRental()])}
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
          Отдельная строка расходов бюджета — «Аренда гаража».
        </Text>
        {renderRentalTable(garages, setGarages, 'например, Гараж №1')}
        {!readonly && (
          <Button
            size="small"
            type="dashed"
            icon={<PlusOutlined />}
            style={{ marginTop: 8 }}
            onClick={() => setGarages(prev => [...prev, emptyRental()])}
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
