import { useCallback, useEffect, useState } from 'react';
import { Card, InputNumber, Typography, Row, Col } from 'antd';
import { useQuery } from '@tanstack/react-query';
import { budgetsApi } from '../../../api';
import type { InputGphEmployees } from '../../../types';
import { monthLabel, thousandFormatter, thousandParser, fmtNum } from '../../../utils/fmt';
import MonthTotals from '../../../components/MonthTotals';
import { titleWithHint } from '../../../components/InfoHint';
import { useAutosave } from '../../../hooks/useAutosave';

const { Text } = Typography;

interface Props {
  versionId: number;
  duration: number;
  startDate?: string;
  readonly?: boolean;
}

/**
 * Форма листа 4.10 «ГПХ сотрудников».
 *
 * Экономист задаёт два числа на весь проект, месяцы заполняются сами:
 * расход каждого месяца = среднее количество × средняя стоимость. Таблица
 * по месяцам здесь только для чтения — вводить в неё нечего.
 */
export default function GphEmployeesInput({ versionId, duration, startDate, readonly }: Props) {
  const [avgCount, setAvgCount] = useState(0);
  const [avgCost, setAvgCost] = useState(0);
  const [hydrated, setHydrated] = useState(false);

  const { data: saved, isSuccess } = useQuery({
    queryKey: ['budget-input', versionId, 'gph_employees'],
    queryFn: () => budgetsApi.getInput<InputGphEmployees>(versionId, 'gph_employees'),
  });

  useEffect(() => {
    if (!isSuccess) return;
    setAvgCount(saved?.avg_count ?? 0);
    setAvgCost(saved?.avg_cost ?? 0);
    setHydrated(true);
  }, [saved, isSuccess]);

  const payload: InputGphEmployees = { avg_count: avgCount, avg_cost: avgCost };

  const save = useCallback(
    (d: InputGphEmployees) => budgetsApi.saveInput(versionId, 'gph_employees', d),
    [versionId],
  );

  useAutosave({ data: payload, ready: hydrated, save, enabled: !readonly });

  const months = Array.from({ length: duration }, (_, i) => monthLabel(startDate, i));
  const perMonth = avgCount * avgCost;
  const values = Array(duration).fill(perMonth) as number[];

  return (
    <Card
      title={titleWithHint(
        'ГПХ сотрудников',
        'Достаточно двух чисел на весь проект: расход каждого месяца '
        + 'считается сам — среднее количество × средняя стоимость. Сумма '
        + 'уходит одной строкой бюджета «Субподрядные работы '
        + '(ГПХ сотрудников)».',
      )}
      size="small"
      extra={<Text type="secondary" style={{ fontSize: 12 }}>
        Итого {fmtNum(perMonth * duration)} ₽
      </Text>}
    >
      <Row gutter={16} style={{ marginBottom: 16 }} align="bottom">
        <Col>
          <Text style={{ fontSize: 12, display: 'block', marginBottom: 4 }}>
            Среднее кол-во в месяц
          </Text>
          {readonly ? (
            <Text strong>{fmtNum(avgCount)}</Text>
          ) : (
            <InputNumber
              style={{ width: 180 }}
              min={0}
              // Среднее, а не штучный счёт: дробное значение допустимо.
              value={avgCount || null}
              placeholder="0"
              onChange={v => setAvgCount(v ?? 0)}
              addonAfter="чел."
            />
          )}
        </Col>
        <Col>
          <Text style={{ fontSize: 12, display: 'block', marginBottom: 4 }}>
            Стоимость в среднем
          </Text>
          {readonly ? (
            <Text strong>{fmtNum(avgCost)} ₽</Text>
          ) : (
            <InputNumber
              style={{ width: 200 }}
              min={0}
              value={avgCost || null}
              placeholder="0"
              onChange={v => setAvgCost(v ?? 0)}
              formatter={thousandFormatter}
              parser={thousandParser}
              addonAfter="₽"
            />
          )}
        </Col>
        <Col>
          <Text type="secondary" style={{ fontSize: 12 }}>
            = {fmtNum(perMonth)} ₽ в месяц
          </Text>
        </Col>
      </Row>

      <MonthTotals months={months} values={values} label="Итого по месяцам" />
    </Card>
  );
}
