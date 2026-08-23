import { useEffect, useState } from 'react';
import { Card, Button, InputNumber, Switch, message, Space, Typography, Alert } from 'antd';
import { SaveOutlined } from '@ant-design/icons';
import { useQuery, useMutation } from '@tanstack/react-query';
import dayjs from 'dayjs';
import { budgetsApi } from '../../../api';
import type { InputRentApartments } from '../../../types';
import { extractError } from '../../../api/client';

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

const cell: React.CSSProperties = {
  border: '1px solid #f0f0f0',
  padding: '2px 4px',
  textAlign: 'center',
};
const headCell: React.CSSProperties = { ...cell, background: '#fafafa', fontWeight: 500 };

export default function RentApartmentsInput({ versionId, duration, startDate, readonly }: Props) {
  const [rows, setRows] = useState<Record<RoomKey, RoomRow>>({
    '1room': emptyRow(duration),
    '2room': emptyRow(duration),
    '3room': emptyRow(duration),
  });
  const [cleaningBase, setCleaningBase] = useState(0);
  const [realtorBase, setRealtorBase] = useState(0);

  const { data: saved } = useQuery({
    queryKey: ['budget-input', versionId, 'rent_apartments'],
    queryFn: () => budgetsApi.getInput<InputRentApartments>(versionId, 'rent_apartments'),
  });

  useEffect(() => {
    if (!saved) return;
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
  }, [saved, duration]);

  const saveMutation = useMutation({
    mutationFn: () => {
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
      };
      return budgetsApi.saveInput(versionId, 'rent_apartments', payload);
    },
    onSuccess: () => message.success('Данные по аренде квартир сохранены'),
    onError: (e) => message.error(extractError(e)),
  });

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

  function setUniform(key: RoomKey, uniform: boolean) {
    setRows(prev => {
      const row = prev[key];
      // При включении режима размножаем первое значение на все месяцы
      const counts = uniform ? Array(duration).fill(row.counts[0] ?? 0) : row.counts;
      return { ...prev, [key]: { ...row, uniform, counts } };
    });
  }

  const months = Array.from({ length: duration }, (_, i) =>
    dayjs(startDate).add(i, 'month').format('MM.YY'));

  return (
    <div>
      <Card
        title="Аренда квартир — лист 4.2"
        size="small"
        style={{ marginBottom: 16 }}
        extra={!readonly && (
          <Button
            size="small"
            type="primary"
            icon={<SaveOutlined />}
            onClick={() => saveMutation.mutate()}
            loading={saveMutation.isPending}
          >
            Сохранить
          </Button>
        )}
      >
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message="Суммы считаются автоматически"
          description={
            'Стоимость аренды = количество × цена + уборка, и уходит в бюджет ' +
            'одной строкой (уборка входит внутрь). Риелтор считается отдельно: ' +
            'в первый месяц — за все квартиры, далее — только за прирост ' +
            'количества. В последний месяц проекта риелтор не начисляется.'
          }
        />

        <div style={{ overflowX: 'auto' }}>
          <table style={{ borderCollapse: 'collapse', fontSize: 13 }}>
            <thead>
              <tr>
                <th style={headCell} rowSpan={2}>Тип</th>
                <th style={headCell} rowSpan={2}>Цена за месяц, ₽</th>
                <th style={headCell} colSpan={duration}>Количество по месяцам</th>
                <th style={headCell} rowSpan={2}>Единое<br />значение</th>
              </tr>
              <tr>
                {months.map((m, i) => (
                  <th key={i} style={{ ...headCell, minWidth: 70, fontWeight: 400, color: '#888' }}>
                    {m}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {ROOM_TYPES.map(({ key, label }) => {
                const row = rows[key];
                return (
                  <tr key={key}>
                    <td style={{ ...cell, fontWeight: 500 }}>{label}</td>
                    <td style={cell}>
                      <InputNumber
                        size="small"
                        min={0}
                        value={row.price}
                        onChange={v => setPrice(key, v ?? 0)}
                        disabled={readonly}
                        style={{ width: 110 }}
                      />
                    </td>
                    {row.counts.map((v, i) => (
                      <td key={i} style={cell}>
                        <InputNumber
                          size="small"
                          min={0}
                          value={v}
                          onChange={val => setCount(key, i, val ?? 0)}
                          // В режиме «единое значение» правим только первую ячейку
                          disabled={readonly || (row.uniform && i > 0)}
                          style={{ width: 56 }}
                        />
                      </td>
                    ))}
                    <td style={cell}>
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
            </tbody>
          </table>
        </div>
        <Text type="secondary" style={{ display: 'block', marginTop: 8, fontSize: 12 }}>
          «Единое значение» — одно количество на все месяцы проекта. Выключено —
          количество задаётся для каждого месяца отдельно.
        </Text>
      </Card>

      <Space size={16} align="start" wrap>
        <Card title="Уборка квартир" size="small">
          <Text type="secondary" style={{ display: 'block', marginBottom: 8, fontSize: 12 }}>
            Стоимость уборки одной квартиры за месяц. Входит в стоимость аренды.
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

        <Card title="Риелтор" size="small">
          <Text type="secondary" style={{ display: 'block', marginBottom: 8, fontSize: 12 }}>
            Стоимость услуг риелтора за одну квартиру. Отдельная строка бюджета.
          </Text>
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
    </div>
  );
}
