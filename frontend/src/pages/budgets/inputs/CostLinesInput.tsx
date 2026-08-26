import { useCallback, useEffect, useState } from 'react';
import { Card, Input, InputNumber, Button, Switch, Typography } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { useQuery } from '@tanstack/react-query';
import { budgetsApi } from '../../../api';
import type { CostLine, InputCostLines } from '../../../types';
import { monthLabel, thousandFormatter, thousandParser, fmtNum } from '../../../utils/fmt';
import MonthGrid, { monthGridCell, monthGridHeadCell } from '../../../components/MonthGrid';
import {
  totalsRow, totalsLabelCell, totalsValueCell, totalsGrandCell,
} from '../../../components/MonthTotals';
import DeleteRowButton from '../../../components/DeleteRowButton';
import { titleWithHint } from '../../../components/InfoHint';
import { useAutosave } from '../../../hooks/useAutosave';

const { Text } = Typography;

const NAME_COL = 220;
// Крайняя колонка держит тумблер, подпись к нему и кнопку удаления, а в
// подвале — итог за проект.
const SWITCH_COL = 175;

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

/**
 * Ячейка тумблера. В отличие от соседних, содержит один элемент, поэтому
 * центрируется по вертикали, а не прижимается к верху: иначе тумблер
 * висел бы выше строки полей, к которой относится.
 */
const switchCell: React.CSSProperties = {
  ...monthGridCell,
  verticalAlign: 'middle',
  padding: '4px',
};

/** Приводит массив сумм к длине проекта; бэкенд поступает так же. */
function padAmounts(amounts: number[] | undefined, duration: number): number[] {
  const out: number[] = Array(duration).fill(0);
  (amounts ?? []).slice(0, duration).forEach((v, i) => { out[i] = v || 0; });
  return out;
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
      title={titleWithHint(title, hint)}
      size="small"
      extra={<Text type="secondary" style={{ fontSize: 12 }}>
        Итого {fmtNum(grandTotal)} ₽
      </Text>}
    >
      {lines.length === 0 ? (
        <Text type="secondary" style={{ fontSize: 12 }}>{emptyLabel}</Text>
      ) : (
        <MonthGrid
          months={months}
          labelWidth={NAME_COL}
          trailingWidth={SWITCH_COL}
          head={(
            <thead>
              <tr>
                <th style={{ ...monthGridHeadCell, textAlign: 'left', color: '#333', fontWeight: 500 }}>
                  {nameLabel}
                </th>
                {months.map((m, i) => <th key={i} style={monthGridHeadCell}>{m}</th>)}
                {/* Колонка тумблеров — без заголовка. */}
                <th style={monthGridHeadCell} />
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
                      <Input
                        size="small"
                        value={l.name}
                        placeholder={namePlaceholder}
                        onChange={e => patch(idx, { name: e.target.value })}
                      />
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
                  {/* Тумблер и удаление — последними в строке, как в
                      таблицах покупок и на листе «Сотрудники». */}
                  <td style={switchCell}>
                    {!readonly && (
                      <div style={{ display: 'flex', alignItems: 'center', gap: 4, justifyContent: 'center' }}>
                        <Switch
                          size="small"
                          checked={isUniform}
                          onChange={c => toggleUniform(idx, c)}
                        />
                        <span style={{ fontSize: 10, color: '#888', fontWeight: 400 }}>одна стоимость</span>
                        <DeleteRowButton
                          onConfirm={() => setLines(prev => prev.filter((_, i) => i !== idx))}
                        />
                      </div>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
          {/* Вид строки итогов — общий с листом 4.7 (MonthTotals): итог за
              весь проект стоит правее всех месяцев, в колонке тумблеров. */}
          <tfoot>
            <tr style={totalsRow}>
              <td style={totalsLabelCell}>Итого за месяц</td>
              {monthTotals.map((v, i) => (
                <td key={i} style={totalsValueCell}>{v ? fmtNum(v) : '—'}</td>
              ))}
              <td style={totalsGrandCell}>{fmtNum(grandTotal)}</td>
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
