import { useCallback, useEffect, useRef, useState } from 'react';
import { Card, InputNumber } from 'antd';
import { useQuery } from '@tanstack/react-query';
import { budgetsApi } from '../../../api';
import { monthLabel, thousandFormatter, thousandParser, fmtNum } from '../../../utils/fmt';
import MonthGrid, { monthGridCell, monthGridHeadCell } from '../../../components/MonthGrid';
import { titleWithHint } from '../../../components/InfoHint';
import { useAutosave } from '../../../hooks/useAutosave';

// Накладные статьи, которые вводятся вручную (строки 186-211 листа 2.Бюджет, исключая рассчитываемые)
const OVERHEAD_LINES: { key: string; label: string }[] = [
  { key: 'overtime_rf',        label: 'Переработки сотрудников РФ' },
  { key: 'overtime_kg',        label: 'Переработки сотрудников КГ' },
  { key: 'internet',           label: 'Интернет' },
  { key: 'mobile',             label: 'Мобильная связь' },
  { key: 'lab_research',       label: 'Лабораторные исследования' },
  { key: 'training',           label: 'Обучение персонала' },
  { key: 'medical',            label: 'Медицинский осмотр' },
  { key: 'uniform',            label: 'Спецодежда' },
  { key: 'computers',          label: 'Приобретение ПК + оргтехника' },
  { key: 'furniture',          label: 'Приобретение мебели' },
  { key: 'office_supplies',    label: 'Содержание офиса (канцтовары)' },
  { key: 'postal',             label: 'Почтовые расходы' },
  { key: 'fuel',               label: 'ГСМ' },
  { key: 'transport_services', label: 'Транспортные услуги' },
  { key: 'subcontract_org',    label: 'Субподряд (орг. улучшения)' },
  { key: 'representative',     label: 'Представительские расходы' },
  { key: 'bank_services',      label: 'Услуги банков' },
  { key: 'insurance_liab',     label: 'Страхование ответственности' },
  { key: 'utilities',          label: 'Коммунальные расходы' },
  { key: 'security',           label: 'Охрана объекта' },
  { key: 'auto_insurance',     label: 'Страхование КАСКО и ОСАГО' },
];

interface Props {
  versionId: number;
  duration: number;
  startDate?: string;
  readonly?: boolean;
}

type OverheadData = Record<string, { monthly_amounts: number[] }>;

export default function OverheadInput({ versionId, duration, startDate, readonly }: Props) {
  const [data, setData] = useState<OverheadData>({});

  // Загружаем все статьи разом
  const { data: allInputs, isLoading, isSuccess } = useQuery({
    queryKey: ['budget-inputs-all', versionId],
    queryFn: () => budgetsApi.getAllInputs(versionId),
  });

  const [hydrated, setHydrated] = useState(false);

  // Каждая статья — отдельная запись budget_inputs, поэтому шлём только
  // изменившиеся: 21 запрос на каждое нажатие клавиши был бы перебором.
  const savedSnapshot = useRef<OverheadData>({});

  useEffect(() => {
    if (!isSuccess || !allInputs) return;
    const next: OverheadData = {};
    for (const line of OVERHEAD_LINES) {
      const raw = allInputs[line.key] as { monthly_amounts?: number[] } | undefined;
      next[line.key] = { monthly_amounts: (raw?.monthly_amounts ?? Array(duration).fill(0)) };
    }
    setData(next);
    savedSnapshot.current = JSON.parse(JSON.stringify(next));
    setHydrated(true);
  }, [allInputs, duration, isSuccess]);

  const save = useCallback(async (current: OverheadData) => {
    for (const line of OVERHEAD_LINES) {
      const next = current[line.key] ?? { monthly_amounts: Array(duration).fill(0) };
      const prev = savedSnapshot.current[line.key];
      if (prev && JSON.stringify(prev) === JSON.stringify(next)) continue;
      await budgetsApi.saveInput(versionId, line.key, next);
      savedSnapshot.current[line.key] = next;
    }
  }, [versionId, duration]);

  useAutosave({ data, ready: hydrated, save, enabled: !readonly });

  function updateAmount(key: string, monthIdx: number, value: number) {
    const amounts = [...(data[key]?.monthly_amounts ?? Array(duration).fill(0))];
    while (amounts.length < duration) amounts.push(0);
    amounts[monthIdx] = value;
    setData({ ...data, [key]: { monthly_amounts: amounts } });
  }

  function lineTotal(key: string): number {
    return (data[key]?.monthly_amounts ?? []).reduce((s, v) => s + (v || 0), 0);
  }

  return (
    <Card
      title={titleWithHint(
        'Прочие накладные расходы',
        'Введите суммы по каждой статье помесячно. Пустые строки не войдут '
        + 'в итоговый расчёт.',
      )}
      size="small"
    >
      <MonthGrid
        months={Array.from({ length: duration }, (_, i) => monthLabel(startDate, i))}
        labelWidth={240}
        trailingWidth={110}
        head={(
          <thead>
            <tr>
              <th style={{ ...monthGridHeadCell, textAlign: 'left', color: '#333', fontWeight: 500 }}>
                Статья
              </th>
              {Array.from({ length: duration }).map((_, i) => (
                <th key={i} style={monthGridHeadCell}>{monthLabel(startDate, i)}</th>
              ))}
              <th style={{ ...monthGridHeadCell, textAlign: 'right', color: '#333', fontWeight: 500 }}>
                Итого
              </th>
            </tr>
          </thead>
        )}
      >
        <tbody>
          {OVERHEAD_LINES.map((line) => (
            <tr key={line.key} style={{ borderTop: '1px solid #f0f0f0' }}>
              <td style={{ ...monthGridCell, textAlign: 'left', fontSize: 12 }}>{line.label}</td>
              {Array.from({ length: duration }).map((_, monthIdx) => {
                const val = data[line.key]?.monthly_amounts?.[monthIdx] ?? 0;
                return (
                  <td key={monthIdx} style={monthGridCell}>
                    {readonly ? (
                      <span style={{ fontSize: 12 }}>{val ? fmtNum(val) : '—'}</span>
                    ) : (
                      <InputNumber
                        size="small"
                        style={{ width: '100%' }}
                        value={val || null}
                        min={0}
                        onChange={(v) => updateAmount(line.key, monthIdx, v ?? 0)}
                        formatter={thousandFormatter}
                        parser={thousandParser}
                      />
                    )}
                  </td>
                );
              })}
              <td style={{ ...monthGridCell, textAlign: 'right', fontWeight: 500, fontSize: 12 }}>
                {fmtNum(lineTotal(line.key))}
              </td>
            </tr>
          ))}
        </tbody>
      </MonthGrid>
    </Card>
  );
}
