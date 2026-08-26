import { useCallback, useEffect, useState } from 'react';
import { Card, InputNumber, Button, Row, Col, Typography, Switch } from 'antd';
import { useQuery } from '@tanstack/react-query';
import { budgetsApi } from '../../../api';
import { monthLabel, thousandFormatter, thousandParser } from '../../../utils/fmt';
import MonthGrid, { monthGridCell, monthGridHeadCell } from '../../../components/MonthGrid';
import { useAutosave } from '../../../hooks/useAutosave';

const { Text } = Typography;

interface Props {
  versionId: number;
  type: string;
  title: string;
  duration: number;
  readonly?: boolean;
  startDate?: string;
}

interface SimpleCostData {
  monthly_amounts: number[];
}

export default function SimpleCostInput({ versionId, type, title, duration, readonly, startDate }: Props) {
  const [amounts, setAmounts] = useState<number[]>(Array(duration).fill(0));
  const [uniformMode, setUniformMode] = useState(false);
  const [uniformValue, setUniformValue] = useState<number>(0);

  const [hydrated, setHydrated] = useState(false);

  const { data, isSuccess } = useQuery({
    queryKey: ['budget-input', versionId, type],
    queryFn: () => budgetsApi.getInput<SimpleCostData>(versionId, type),
  });

  useEffect(() => {
    if (!isSuccess) return;
    if (data?.monthly_amounts) {
      const arr = Array(duration).fill(0);
      data.monthly_amounts.forEach((v, i) => { if (i < duration) arr[i] = v; });
      setAmounts(arr);
    }
    setHydrated(true);
  }, [data, duration, isSuccess]);

  const save = useCallback(
    (amts: number[]) => budgetsApi.saveInput(versionId, type, { monthly_amounts: amts }),
    [versionId, type],
  );

  useAutosave({ data: amounts, ready: hydrated, save, enabled: !readonly });

  function applyUniform() {
    setAmounts(Array(duration).fill(uniformValue));
  }

  const total = amounts.reduce((s, v) => s + (v || 0), 0);

  return (
    <Card title={title} size="small">
      {!readonly && (
        <Row align="middle" gutter={8} style={{ marginBottom: 16 }}>
          <Col>
            <Switch
              size="small"
              checked={uniformMode}
              onChange={setUniformMode}
            />
          </Col>
          <Col><Text type="secondary">Единое значение на весь период</Text></Col>
          {uniformMode && (
            <>
              <Col>
                <InputNumber
                  style={{ width: 160 }}
                  value={uniformValue}
                  onChange={(v) => setUniformValue(v ?? 0)}
                  min={0}
                  formatter={thousandFormatter}
                  parser={thousandParser}
                  addonAfter="₽"
                />
              </Col>
              <Col>
                <Button size="small" onClick={applyUniform}>Применить</Button>
              </Col>
            </>
          )}
        </Row>
      )}

      <MonthGrid
        months={amounts.map((_, i) => monthLabel(startDate, i))}
        trailingWidth={130}
        head={(
          <thead>
            <tr>
              {amounts.map((_, i) => (
                <th key={i} style={monthGridHeadCell}>{monthLabel(startDate, i)}</th>
              ))}
              <th style={{ ...monthGridHeadCell, fontWeight: 600, color: '#333', textAlign: 'right' }}>
                Итого
              </th>
            </tr>
          </thead>
        )}
      >
        <tbody>
          <tr>
            {amounts.map((v, i) => (
              <td key={i} style={monthGridCell}>
                {readonly ? (
                  <Text>{v ? v.toLocaleString('ru-RU') : '—'}</Text>
                ) : (
                  <InputNumber
                    size="small"
                    style={{ width: '100%' }}
                    value={v || null}
                    min={0}
                    onChange={(val) => {
                      const next = [...amounts];
                      next[i] = val ?? 0;
                      setAmounts(next);
                    }}
                    formatter={thousandFormatter}
                    parser={thousandParser}
                  />
                )}
              </td>
            ))}
            <td style={{ ...monthGridCell, textAlign: 'right', fontWeight: 600 }}>
              {total.toLocaleString('ru-RU')} ₽
            </td>
          </tr>
        </tbody>
      </MonthGrid>
    </Card>
  );
}
