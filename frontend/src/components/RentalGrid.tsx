import { useState } from 'react';
import { Input, InputNumber, Button, Switch, Typography } from 'antd';
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import type { RentedItem } from '../types';
import { thousandFormatter, thousandParser } from '../utils/fmt';
import MonthGrid, { monthGridCell, monthGridHeadCell } from './MonthGrid';

const { Text } = Typography;

const NAME_COL = 200;
const TOTAL_COL = 110;

/**
 * Ячейка месяца. Левая колонка выше остальных — в ней название и цена в
 * две строки, — поэтому содержимое прижимаем к верху, чтобы количества
 * стояли вровень с названием, а не по центру строки. Паддинг тот же, что
 * у левой ячейки, иначе поля разъезжаются на пару пикселей.
 */
const monthCountCell: React.CSSProperties = {
  ...monthGridCell,
  verticalAlign: 'top',
  padding: '4px',
};

/**
 * Приводит массив количеств к длине проекта: короткий добивается нулями,
 * длинный обрезается. Нужно после смены длительности проекта у уже
 * сохранённой версии — бэкенд поступает так же.
 */
export function padCounts(counts: number[] | undefined, duration: number): number[] {
  const out = Array(duration).fill(0);
  (counts ?? []).slice(0, duration).forEach((v, i) => { out[i] = v || 0; });
  return out;
}

export function emptyRental(duration: number): RentedItem {
  return { name: '', price: 0, counts: Array(duration).fill(0) };
}

/** Итог строки: цена × количество, просуммированное по месяцам. */
export function rentalTotal(r: RentedItem, duration: number): number {
  return padCounts(r.counts, duration).reduce((s, c) => s + r.price * c, 0);
}

interface Props {
  items: RentedItem[];
  onChange: (next: RentedItem[]) => void;
  /** Подписи месяцев проекта по порядку. */
  months: string[];
  readonly?: boolean;
  /** Заголовок левой колонки, например «Вид / цена за ед. в месяц, ₽». */
  headLabel: string;
  namePlaceholder: string;
  addLabel: string;
  emptyLabel: string;
}

/**
 * Таблица помесячной аренды: строка = один арендуемый вид со своей ценой,
 * колонки = месяцы с количеством. Расход месяца = цена × количество,
 * просуммированное по строкам.
 *
 * Повторяет структуру формы: цена за единицу плюс количество, заданное
 * отдельно в каждом месяце — 4.3!B6 + C5:BJ5 (авто), 4.3!B11 + C10:BJ10
 * (гараж), 4.5!B15:B17 + C15:BJ17 (офисы). Единственное расширение —
 * строк может быть несколько, у каждой своя цена.
 */
export default function RentalGrid({
  items, onChange, months, readonly, headLabel, namePlaceholder, addLabel, emptyLabel,
}: Props) {
  const duration = months.length;
  // «Единое значение на все месяцы» — только режим ввода, на сервер не идёт.
  const [uniform, setUniform] = useState<boolean[]>([]);

  function patch(idx: number, p: Partial<RentedItem>) {
    onChange(items.map((r, i) => (i === idx ? { ...r, ...p } : r)));
  }

  function setCount(idx: number, isUniform: boolean, monthIdx: number, value: number) {
    onChange(items.map((r, i) => {
      if (i !== idx) return r;
      const counts = isUniform
        ? Array(duration).fill(value)
        : padCounts(r.counts, duration).map((v, m) => (m === monthIdx ? value : v));
      return { ...r, counts };
    }));
  }

  function toggleUniform(idx: number, on: boolean) {
    setUniform(prev => {
      const next = [...prev];
      next[idx] = on;
      return next;
    });
    if (!on) return;
    // При включении первое количество размножается на все месяцы.
    onChange(items.map((r, i) => (
      i === idx ? { ...r, counts: Array(duration).fill(padCounts(r.counts, duration)[0]) } : r
    )));
  }

  return (
    <div>
      {items.length === 0 ? (
        <Text type="secondary" style={{ fontSize: 12 }}>{emptyLabel}</Text>
      ) : (
        <MonthGrid
          months={months}
          labelWidth={NAME_COL}
          trailingWidth={TOTAL_COL}
          head={(
            <thead>
              <tr>
                <th style={{ ...monthGridHeadCell, textAlign: 'left', color: '#333', fontWeight: 500 }}>
                  {headLabel}
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
              const isUniform = !!uniform[idx];
              return (
                <tr key={idx} style={{ borderTop: '1px solid #f0f0f0' }}>
                  <td style={{ ...monthCountCell, textAlign: 'left' }}>
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
                          onChange={e => patch(idx, { name: e.target.value })}
                        />
                        <div style={{ display: 'flex', gap: 4, alignItems: 'center' }}>
                          <InputNumber
                            size="small"
                            style={{ width: '100%' }}
                            min={0}
                            value={r.price || null}
                            placeholder="цена"
                            onChange={v => patch(idx, { price: v ?? 0 })}
                            formatter={thousandFormatter}
                            parser={thousandParser}
                          />
                          <Button
                            size="small"
                            type="text"
                            danger
                            icon={<DeleteOutlined />}
                            onClick={() => onChange(items.filter((_, i) => i !== idx))}
                          />
                        </div>
                      </div>
                    )}
                  </td>
                  {counts.map((v, i) => (
                    <td key={i} style={monthCountCell}>
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
                          onChange={val => setCount(idx, isUniform, i, val ?? 0)}
                        />
                      )}
                    </td>
                  ))}
                  <td style={{ ...monthCountCell, textAlign: 'right', fontWeight: 500, fontSize: 12 }}>
                    <div>{rentalTotal(r, duration).toLocaleString('ru-RU')}</div>
                    {!readonly && (
                      <div style={{ display: 'flex', alignItems: 'center', gap: 4, justifyContent: 'flex-end' }}>
                        <Switch
                          size="small"
                          checked={isUniform}
                          onChange={c => toggleUniform(idx, c)}
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
      )}
      {!readonly && (
        <Button
          size="small"
          type="dashed"
          icon={<PlusOutlined />}
          style={{ marginTop: 8 }}
          onClick={() => onChange([...items, emptyRental(duration)])}
        >
          {addLabel}
        </Button>
      )}
    </div>
  );
}
