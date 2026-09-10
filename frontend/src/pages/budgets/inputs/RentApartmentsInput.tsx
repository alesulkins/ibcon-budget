import { useCallback, useEffect, useState } from 'react';
import { Card, InputNumber, Switch, Space, Typography, Checkbox, Button } from 'antd';
import { useQuery } from '@tanstack/react-query';
import { budgetsApi } from '../../../api';
import type { InputRentApartments } from '../../../types';
import { monthLabel } from '../../../utils/fmt';
import MonthGrid, {
  monthGridCell, monthGridHeadCell, LABEL_COL_WIDTH,
} from '../../../components/MonthGrid';
import { titleWithHint } from '../../../components/InfoHint';
import { useAutosave } from '../../../hooks/useAutosave';
import RentMarketModal from './RentMarketModal';

const { Text } = Typography;

interface Props {
  versionId: number;
  duration: number;
  startDate: string;
  readonly?: boolean;
}

/** Типы квартир: ключи полей совпадают с InputRentApartments. */
const ROOM_TYPES = [
  { key: '1room', label: '1кк' },
  { key: '2room', label: '2кк' },
  { key: '3room', label: '3кк' },
] as const;

type RoomKey = (typeof ROOM_TYPES)[number]['key'];

/** Локальное состояние одной строки таблицы. */
interface RoomRow {
  price: number;
  counts: number[];
  /** true — одно количество на все месяцы, false — по каждому месяцу (по умолчанию) */
  uniform: boolean;
}

function emptyRow(duration: number): RoomRow {
  return { price: 0, counts: Array(duration).fill(0), uniform: false };
}

export default function RentApartmentsInput({
  versionId, duration, startDate, readonly,
}: Props) {
  // Запрос рыночной стоимости к площадкам объявлений — по кнопке, а не
  // при открытии шага: поход к площадкам занимает секунды и цену всё
  // равно ставит человек.
  const [marketOpen, setMarketOpen] = useState(false);
  const [rows, setRows] = useState<Record<RoomKey, RoomRow>>({
    '1room': emptyRow(duration),
    '2room': emptyRow(duration),
    '3room': emptyRow(duration),
  });
  const [cleaningBase, setCleaningBase] = useState(0);
  const [realtorBase, setRealtorBase] = useState(0);
  // Месяцы (1-based), в которых начисляется уборка. Пусто — уборки нет.
  const [cleaningMonths, setCleaningMonths] = useState<number[]>([]);

  const { data: saved, isSuccess } = useQuery({
    queryKey: ['budget-input', versionId, 'rent_apartments'],
    queryFn: () => budgetsApi.getInput<InputRentApartments>(versionId, 'rent_apartments'),
  });

  const [hydrated, setHydrated] = useState(false);

  useEffect(() => {
    if (!isSuccess) return;
    if (!saved) { setHydrated(true); return; }
    const pick = (arr: number[] | undefined): number[] => {
      const out = Array(duration).fill(0);
      (arr ?? []).slice(0, duration).forEach((v, i) => { out[i] = v; });
      return out;
    };
    setRows({
      '1room': { price: saved.price_1room ?? 0, counts: pick(saved.count_1room), uniform: false },
      '2room': { price: saved.price_2room ?? 0, counts: pick(saved.count_2room), uniform: false },
      '3room': { price: saved.price_3room ?? 0, counts: pick(saved.count_3room), uniform: false },
    });
    setCleaningBase(saved.cleaning_base ?? 0);
    setRealtorBase(saved.realtor_base ?? 0);
    setCleaningMonths(saved.cleaning_months ?? []);
    setHydrated(true);
  }, [saved, duration, isSuccess]);

  // Передаём количества и цены. Аренду (с уборкой) и риелтора
  // считает бэкенд — calcRentApartments, лист 4.2.
  const payload: InputRentApartments = {
    price_1room: rows['1room'].price,
    price_2room: rows['2room'].price,
    price_3room: rows['3room'].price,
    count_1room: rows['1room'].counts,
    count_2room: rows['2room'].counts,
    count_3room: rows['3room'].counts,
    cleaning_base: cleaningBase,
    realtor_base: realtorBase,
    cleaning_months: [...cleaningMonths].sort((a, b) => a - b),
  };

  const save = useCallback(
    (d: InputRentApartments) => budgetsApi.saveInput(versionId, 'rent_apartments', d),
    [versionId],
  );

  useAutosave({ data: payload, ready: hydrated, save, enabled: !readonly });

  function setPrice(key: RoomKey, price: number) {
    setRows(prev => ({ ...prev, [key]: { ...prev[key], price } }));
  }

  function setCount(key: RoomKey, monthIdx: number, value: number) {
    setRows(prev => {
      const row = prev[key];
      // В режиме «единое значение» число проставляется на все месяцы сразу
      const counts = row.uniform
        ? Array(duration).fill(value)
        : row.counts.map((v, i) => (i === monthIdx ? value : v));
      return { ...prev, [key]: { ...row, counts } };
    });
  }

  function toggleCleaningMonth(monthNum: number, on: boolean) {
    setCleaningMonths(prev => on
      ? [...prev, monthNum]
      : prev.filter(m => m !== monthNum));
  }

  function setUniform(key: RoomKey, uniform: boolean) {
    setRows(prev => {
      const row = prev[key];
      // При включении режима размножаем первое значение на все месяцы
      const counts = uniform ? Array(duration).fill(row.counts[0] ?? 0) : row.counts;
      return { ...prev, [key]: { ...row, uniform, counts } };
    });
  }

  const months = Array.from({ length: duration }, (_, i) => monthLabel(startDate, i));

  return (
    <div>
      <Card
        title={titleWithHint(
          'Аренда квартир',
          '«Единое значение» — одно количество на все месяцы проекта. '
          + 'Выключено — количество задаётся для каждого месяца отдельно. '
          + 'Уборка начисляется только в отмеченных месяцах; если не отмечен '
          + 'ни один — уборка за весь период равна нулю.',
        )}
        size="small"
        style={{ marginBottom: 16 }}
        extra={(
          <Button size="small" onClick={() => setMarketOpen(true)}>
            Узнать рыночную стоимость
          </Button>
        )}
      >
        <MonthGrid
          months={months}
          labelWidth={LABEL_COL_WIDTH}
          trailingWidth={90}
          head={(
            <thead>
              <tr>
                <th style={{ ...monthGridHeadCell, textAlign: 'left' }}>Тип / цена, ₽</th>
                {months.map((m, i) => (
                  <th key={i} style={monthGridHeadCell}>{m}</th>
                ))}
                <th style={monthGridHeadCell}>Единое<br />значение</th>
              </tr>
            </thead>
          )}
        >
          <tbody>
            {ROOM_TYPES.map(({ key, label }) => {
              const row = rows[key];
              return (
                <tr key={key}>
                  <td style={{ ...monthGridCell, textAlign: 'left' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                      <span style={{ fontWeight: 500, fontSize: 13 }}>{label}</span>
                      <InputNumber
                        size="small"
                        min={0}
                        value={row.price}
                        onChange={v => setPrice(key, v ?? 0)}
                        disabled={readonly}
                        style={{ width: '100%' }}
                      />
                    </div>
                  </td>
                  {row.counts.map((v, i) => (
                    <td key={i} style={monthGridCell}>
                      <InputNumber
                        size="small"
                        min={0}
                        value={v}
                        onChange={val => setCount(key, i, val ?? 0)}
                        // В режиме «единое значение» правим только первую ячейку
                        disabled={readonly || (row.uniform && i > 0)}
                        style={{ width: '100%' }}
                      />
                    </td>
                  ))}
                  <td style={monthGridCell}>
                    <Switch
                      size="small"
                      checked={row.uniform}
                      onChange={c => setUniform(key, c)}
                      disabled={readonly}
                    />
                  </td>
                </tr>
              );
            })}

            {/* Месяцы, в которых начисляется уборка */}
            <tr>
              <td style={{ ...monthGridCell, textAlign: 'left', fontSize: 12, paddingTop: 12 }}>
                Уборка в месяце
              </td>
              {months.map((_, i) => (
                <td key={i} style={{ ...monthGridCell, paddingTop: 12 }}>
                  <Checkbox
                    checked={cleaningMonths.includes(i + 1)}
                    disabled={readonly}
                    onChange={e => toggleCleaningMonth(i + 1, e.target.checked)}
                  />
                </td>
              ))}
              <td style={{ ...monthGridCell, paddingTop: 12 }}>
                {!readonly && (
                  <Button
                    size="small"
                    type="link"
                    style={{ padding: 0, fontSize: 12 }}
                    onClick={() => setCleaningMonths(
                      cleaningMonths.length === duration
                        ? []
                        : Array.from({ length: duration }, (_, i) => i + 1),
                    )}
                  >
                    {cleaningMonths.length === duration ? 'снять' : 'все'}
                  </Button>
                )}
              </td>
            </tr>
          </tbody>
        </MonthGrid>
      </Card>

      <Space size={16} align="start" wrap>
        <Card title="Уборка квартир" size="small">
          <Text type="secondary" style={{ display: 'block', marginBottom: 8, fontSize: 12 }}>
            Стоимость уборки одной квартиры за месяц. Входит в стоимость аренды.
            Месяцы начисления отмечаются в таблице выше
            {cleaningMonths.length === 0
              ? ' — сейчас не отмечен ни один, уборка не начисляется.'
              : ` — отмечено месяцев: ${cleaningMonths.length}.`}
          </Text>
          <InputNumber
            min={0}
            value={cleaningBase}
            onChange={v => setCleaningBase(v ?? 0)}
            disabled={readonly}
            style={{ width: 200 }}
            addonAfter="₽"
          />
        </Card>

        <Card
          title={titleWithHint(
            'Риелтор',
            'Стоимость услуг риелтора за одну квартиру. Отдельная строка бюджета.',
          )}
          size="small"
        >
          <InputNumber
            min={0}
            value={realtorBase}
            onChange={v => setRealtorBase(v ?? 0)}
            disabled={readonly}
            style={{ width: 200 }}
            addonAfter="₽"
          />
        </Card>
      </Space>

      <RentMarketModal
        open={marketOpen}
        onClose={() => setMarketOpen(false)}
        // Подстановка только когда версию можно править: в архивной
        // версии кнопка «узнать» остаётся, а «подставить» — нет.
        onApply={readonly
          ? undefined
          : (r, price) => setPrice(`${r}room` as RoomKey, price)}
      />
    </div>
  );
}
