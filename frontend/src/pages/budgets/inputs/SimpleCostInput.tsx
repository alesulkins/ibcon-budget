import React, { useEffect, useState } from 'react';
import {
  Card, InputNumber, Button, Row, Col, Typography, Space, message,
  Switch, Tooltip,
} from 'antd';
import { SaveOutlined, InfoCircleOutlined } from '@ant-design/icons';
import { useQuery, useMutation } from '@tanstack/react-query';
import dayjs from 'dayjs';
import { budgetsApi } from '../../../api';
import { extractError } from '../../../api/client';

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

  const { data } = useQuery({
    queryKey: ['budget-input', versionId, type],
    queryFn: () => budgetsApi.getInput<SimpleCostData>(versionId, type),
  });

  useEffect(() => {
    if (data?.monthly_amounts) {
      const arr = Array(duration).fill(0);
      data.monthly_amounts.forEach((v, i) => { if (i < duration) arr[i] = v; });
      setAmounts(arr);
    }
  }, [data, duration]);

  const saveMutation = useMutation({
    mutationFn: () => budgetsApi.saveInput(versionId, type, { monthly_amounts: amounts }),
    onSuccess: () => message.success('Данные сохранены'),
    onError: (e) => message.error(extractError(e)),
  });

  function applyUniform() {
    setAmounts(Array(duration).fill(uniformValue));
  }

  function monthLabel(idx: number): string {
    if (!startDate) return `М${idx + 1}`;
    const d = dayjs(startDate).add(idx, 'month');
    return d.format('MMM YY');
  }

  const total = amounts.reduce((s, v) => s + (v || 0), 0);

  return (
    <Card
      title={title}
      size="small"
      extra={
        !readonly && (
          <Button
            type="primary"
            icon={<SaveOutlined />}
            onClick={() => saveMutation.mutate()}
            loading={saveMutation.isPending}
            size="small"
            style={{ background: '#1a3a6b' }}
          >
            Сохранить
          </Button>
        )
      }
    >
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
                  formatter={(v) => `${v}`.replace(/\B(?=(\d{3})+(?!\d))/g, ' ')}
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

      <div style={{ overflowX: 'auto' }}>
        <table style={{ borderCollapse: 'collapse', width: '100%' }}>
          <thead>
            <tr>
              {amounts.map((_, i) => (
                <th key={i} style={{ padding: '4px 8px', fontWeight: 400, color: '#888', fontSize: 12, textAlign: 'center', minWidth: 90 }}>
                  {monthLabel(i)}
                </th>
              ))}
              <th style={{ padding: '4px 8px', fontWeight: 600, color: '#333', textAlign: 'right', minWidth: 120 }}>
                Итого
              </th>
            </tr>
          </thead>
          <tbody>
            <tr>
              {amounts.map((v, i) => (
                <td key={i} style={{ padding: '4px 4px', textAlign: 'center' }}>
                  {readonly ? (
                    <Text>{v ? v.toLocaleString('ru-RU') : '—'}</Text>
                  ) : (
                    <InputNumber
                      size="small"
                      style={{ width: 90 }}
                      value={v || undefined}
                      min={0}
                      onChange={(val) => {
                        const next = [...amounts];
                        next[i] = val ?? 0;
                        setAmounts(next);
                      }}
                      formatter={(val) => `${val}`.replace(/\B(?=(\d{3})+(?!\d))/g, ' ')}
                    />
                  )}
                </td>
              ))}
              <td style={{ padding: '4px 8px', textAlign: 'right', fontWeight: 600 }}>
                {total.toLocaleString('ru-RU')} ₽
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </Card>
  );
}
