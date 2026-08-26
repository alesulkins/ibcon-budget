import { useCallback, useEffect, useState } from 'react';
import { Card, Input, InputNumber, Button, Switch, Typography } from 'antd';
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import { useQuery } from '@tanstack/react-query';
import { budgetsApi } from '../../../api';
import type { CostLine, InputCostLines } from '../../../types';
import { monthLabel, thousandFormatter, thousandParser, fmtNum } from '../../../utils/fmt';
import MonthGrid, { monthGridCell, monthGridHeadCell } from '../../../components/MonthGrid';
import { useAutosave } from '../../../hooks/useAutosave';

const { Text } = Typography;

const NAME_COL = 220;
const TOTAL_COL = 140;

/**
 * Ячейка месяца. Левая колонка выше остальных — в ней наименование и
 * кнопка удаления, — поэтому содержимое прижимаем к верху, чтобы поля
 * стояли вровень с наименованием. Так же сделано в RentalGrid.
 */
const cell: React.CSSProperties = {
  ...monthGridCell,
  verticalAlign: 'top',
  padding: '4px',
};

/** Приводит массив сумм к длине проекта; бэкенд поступает так же. */
function padAmounts(amounts: number[] | undefined, duration: number): number[] {
  const out: number[] = Array(duration).fill(0);
  (amounts ?? []).slice(0, duration).forEach((v, i) => { out[i] = v || 0; });
  return out;
}

function lineTotal(l: CostLine, duration: number): number {
  return padAmounts(l.monthly_amounts, duration).reduce((s, v) => s + v, 0);
}

interface Props {
  versionId: number;
  /** Ключ ввода в budget_inputs: software_items и т.п. */
  type: string;
  title: string;
  duration: number;
  startDate?: string;
  readonly?: boolean;
  /** Заголовок левой колонки: «Наименование ПО», «Контрагент / услуга». */
  nameLabel: string;
  namePlaceholder: string;
  addLabel: string;
  emptyLabel: string;
  /** Пояснение под заголовком: куда сумма уходит в бюджете. */
  hint: string;
}

/**
 * Форма листа-списка: позиции с наименованием и стоимостью, введённой
 * отдельно в каждом месяце. Итог месяца — сумма по всем позициям, уходит
 * одной строкой бюджета.
 *
 * Общая для 4.8 (ПО), 4.9 (ГПХ внешний) и 4.11 (субподряд): листы
 * устроены одинаково, различаются только подписями.
 */
export default function CostLinesInput({
  versionId, type, title, duration, startDate, readonly,
  nameLabel, namePlaceholder, addLabel, emptyLabel, hint,
}: Props) {
  const [lines, setLines] = useState<CostLine[]>([]);
  const [hydrated, setHydrated] = useState(false);
  // «Одна стоимость на все месяцы» — только режим ввода, на сервер не идёт.
  const [uniform, setUniform] = useState<boolean[]>([]);

  const { data: saved, isSuccess } = useQuery({
    queryKey: ['budget-input', versionId, type],
    queryFn: () => budgetsApi.getInput<InputCostLines>(versionId, type),
  });

  useEffect(() => {
    if (!isSuccess) return;
    setLines((saved?.lines ?? []).map(l => ({
      ...l,
      monthly_amounts: padAmounts(l.monthly_amounts, duration),
    })));
    setHydrated(true);
  }, [saved, isSuccess, duration]);

  // Пустая строка — обычное состояние формы во время заполнения, но
  // сохранять её незачем.
  const keep = (l: CostLine) =>
    l.name !== '' || padAmounts(l.monthly_amounts, duration).some(v => v !== 0);

  const payload: InputCostLines = {
    lines: lines.filter(keep).map(l => ({
      ...l,
      monthly_amounts: padAmounts(l.monthly_amounts, duration),
    })),
  };

  const save = useCallback(
    (d: InputCostLines) => budgetsApi.saveInput(versionId, type, d),
    [versionId, type],
  );

  useAutosave({ data: payload, ready: hydrated, save, enabled: !readonly });

  const months = Array.from({ length: duration }, (_, i) => monthLabel(startDate, i));

  function patch(idx: number, p: Partial<CostLine>) {
    setLines(prev => prev.map((l, i) => (i === idx ? { ...l, ...p } : l)));
  }

  function setAmount(idx: number, isUniform: boolean, monthIdx: number, value: number) {
    setLines(prev => prev.map((l, i) => {
      if (i !== idx) return l;
      const monthly_amounts = isUniform
        ? Array(duration).fill(value)
        : padAmounts(l.monthly_amounts, duration).map((v, m) => (m === monthIdx ? value : v));
      return { ...l, monthly_amounts };
    }));
  }

  function toggleUniform(idx: number, on: boolean) {
    setUniform(prev => {
      const next = [...prev];
      next[idx] = on;
      return next;
    });
    if (!on) return;
    // При включении первая стоимость размножается на все месяцы.
    setLines(prev => prev.map((l, i) => (
      i === idx
        ? { ...l, monthly_amounts: Array(duration).fill(padAmounts(l.monthly_amounts, duration)[0]) }
        : l
    )));
  }

  // Итог месяца по всем позициям — ровно то, что считает calcCostLines.
  const monthTotals = Array.from({ length: duration }, (_, m) =>
    lines.reduce((s, l) => s + padAmounts(l.monthly_amounts, duration)[m], 0));
  const grandTotal = monthTotals.reduce((s, v) => s + v, 0);

  return (
    <Card
      title={title}
      size="small"
      extra={<Text type="secondary" style={{ fontSize: 12 }}>
        Итого {fmtNum(grandTotal)} ₽
      </Text>}
    >
      <Text type="secondary" style={{ display: 'block', marginBottom: 8, fontSize: 12 }}>
        {hint}
      </Text>

      {lines.length === 0 ? (
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
                  {nameLabel}
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
            {lines.map((l, idx) => {
              const amounts = padAmounts(l.monthly_amounts, duration);
              const isUniform = !!uniform[idx];
              return (
                <tr key={idx} style={{ borderTop: '1px solid #f0f0f0' }}>
                  <td style={{ ...cell, textAlign: 'left' }}>
                    {readonly ? (
                      <Text style={{ fontSize: 12 }}>{l.name || '—'}</Text>
                    ) : (
                      <div style={{ display: 'flex', gap: 4, alignItems: 'center' }}>
                        <Input
                          size="small"
                          value={l.name}
                          placeholder={namePlaceholder}
                          onChange={e => patch(idx, { name: e.target.value })}
                        />
                        <Button
                          size="small"
                          type="text"
                          danger
                          icon={<DeleteOutlined />}
                          onClick={() => setLines(prev => prev.filter((_, i) => i !== idx))}
                        />
                      </div>
                    )}
                  </td>
                  {amounts.map((v, i) => (
                    <td key={i} style={cell}>
                      {readonly ? (
                        <Text style={{ fontSize: 12 }}>{v ? fmtNum(v) : '—'}</Text>
                      ) : (
                        <InputNumber
                          size="small"
                          style={{ width: '100%' }}
                          min={0}
                          // Пусто = 0: позиция есть, но в этом месяце не оплачивается.
                          value={v || null}
                          placeholder="0"
                          // В режиме «одна стоимость» правим только первую ячейку.
                          disabled={isUniform && i > 0}
                          onChange={val => setAmount(idx, isUniform, i, val ?? 0)}
                          formatter={thousandFormatter}
                          parser={thousandParser}
                        />
                      )}
                    </td>
                  ))}
                  <td style={{ ...cell, textAlign: 'right', fontWeight: 500, fontSize: 12 }}>
                    <div>{fmtNum(lineTotal(l, duration))}</div>
                    {!readonly && (
                      <div style={{ display: 'flex', alignItems: 'center', gap: 4, justifyContent: 'flex-end' }}>
                        <Switch
                          size="small"
                          checked={isUniform}
                          onChange={c => toggleUniform(idx, c)}
                        />
                        <span style={{ fontSize: 10, color: '#888', fontWeight: 400 }}>одна стоимость</span>
                      </div>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
          <tfoot>
            <tr style={{ borderTop: '2px solid #e8e8e8', background: '#fafafa' }}>
              <td style={{ ...monthGridCell, textAlign: 'left', fontWeight: 600, fontSize: 12 }}>
                Итого за месяц
              </td>
              {monthTotals.map((v, i) => (
                <td key={i} style={{ ...monthGridCell, fontWeight: 500, fontSize: 12 }}>
                  {v ? fmtNum(v) : '—'}
                </td>
              ))}
              <td style={{ ...monthGridCell, textAlign: 'right', fontWeight: 700, fontSize: 12 }}>
                {fmtNum(grandTotal)}
              </td>
            </tr>
          </tfoot>
        </MonthGrid>
      )}

      {!readonly && (
        <Button
          size="small"
          type="dashed"
          icon={<PlusOutlined />}
          style={{ marginTop: 8 }}
          onClick={() => setLines(prev => [
            ...prev,
            { name: '', monthly_amounts: Array(duration).fill(0) },
          ])}
        >
          {addLabel}
        </Button>
      )}
    </Card>
  );
}
