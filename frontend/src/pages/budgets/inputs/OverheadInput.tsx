import React, { useEffect, useState } from 'react';
import { Card, Button, message, Typography, Space } from 'antd';
import { SaveOutlined } from '@ant-design/icons';
import { useQuery, useMutation } from '@tanstack/react-query';
import { budgetsApi } from '../../../api';
import { extractError } from '../../../api/client';

const { Text } = Typography;

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
  readonly?: boolean;
}

type OverheadData = Record<string, { monthly_amounts: number[] }>;

export default function OverheadInput({ versionId, duration, readonly }: Props) {
  const [data, setData] = useState<OverheadData>({});

  // Загружаем все статьи разом
  const { data: allInputs, isLoading } = useQuery({
    queryKey: ['budget-inputs-all', versionId],
    queryFn: () => budgetsApi.getAllInputs(versionId),
  });

  useEffect(() => {
    if (!allInputs) return;
    const next: OverheadData = {};
    for (const line of OVERHEAD_LINES) {
      const raw = allInputs[line.key] as { monthly_amounts?: number[] } | undefined;
      next[line.key] = { monthly_amounts: (raw?.monthly_amounts ?? Array(duration).fill(0)) };
    }
    setData(next);
  }, [allInputs, duration]);

  const saveMutation = useMutation({
    mutationFn: async () => {
      // Сохраняем каждую статью отдельно
      for (const line of OVERHEAD_LINES) {
        await budgetsApi.saveInput(versionId, line.key, data[line.key] ?? { monthly_amounts: Array(duration).fill(0) });
      }
    },
    onSuccess: () => message.success('Прочие расходы сохранены'),
    onError: (e) => message.error(extractError(e)),
  });

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
      title="Прочие накладные расходы"
      size="small"
      extra={
        !readonly && (
          <Button
            size="small"
            icon={<SaveOutlined />}
            onClick={() => saveMutation.mutate()}
            loading={saveMutation.isPending}
            style={{ background: '#1a3a6b' }}
            type="primary"
          >
            Сохранить всё
          </Button>
        )
      }
    >
      <Text type="secondary" style={{ display: 'block', marginBottom: 12 }}>
        Введите суммы по каждой статье помесячно. Пустые строки не войдут в итоговый расчёт.
      </Text>
      <div style={{ overflowX: 'auto' }}>
        <table style={{ borderCollapse: 'collapse', width: '100%', fontSize: 12 }}>
          <thead>
            <tr>
              <th style={{ padding: '4px 8px', textAlign: 'left', minWidth: 220 }}>Статья</th>
              {Array.from({ length: duration }).map((_, i) => (
                <th key={i} style={{ padding: '4px 6px', textAlign: 'center', minWidth: 80, color: '#888', fontWeight: 400 }}>
                  М{i + 1}
                </th>
              ))}
              <th style={{ padding: '4px 8px', textAlign: 'right', minWidth: 100 }}>Итого</th>
            </tr>
          </thead>
          <tbody>
            {OVERHEAD_LINES.map((line) => (
              <tr key={line.key} style={{ borderTop: '1px solid #f0f0f0' }}>
                <td style={{ padding: '4px 8px', fontWeight: 400 }}>{line.label}</td>
                {Array.from({ length: duration }).map((_, monthIdx) => {
                  const val = data[line.key]?.monthly_amounts?.[monthIdx] ?? 0;
                  return (
                    <td key={monthIdx} style={{ padding: '2px 4px', textAlign: 'center' }}>
                      {readonly ? (
                        <span>{val ? val.toLocaleString('ru-RU') : '—'}</span>
                      ) : (
                        <input
                          type="number"
                          style={{ width: 76, border: '1px solid #d9d9d9', borderRadius: 4, padding: '2px 4px', fontSize: 12, textAlign: 'right' }}
                          value={val || ''}
                          min={0}
                          onChange={(e) => updateAmount(line.key, monthIdx, Number(e.target.value) || 0)}
                        />
                      )}
                    </td>
                  );
                })}
                <td style={{ padding: '4px 8px', textAlign: 'right', fontWeight: 500 }}>
                  {lineTotal(line.key).toLocaleString('ru-RU')}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Card>
  );
}
