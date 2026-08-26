import React from 'react';
import {
  Card, Button, Row, Col, Statistic, Table, Typography, Space,
  message, Spin, Tag,
} from 'antd';
import { CalculatorOutlined, FileExcelOutlined } from '@ant-design/icons';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import type { ColumnsType } from 'antd/es/table';
import { budgetsApi } from '../../api';
import type { MonthlyResult } from '../../types';
import { fmtMoney, monthLabel, fmtNum } from '../../utils/fmt';
import { profitabilityGrade } from '../../utils/profitability';
import Profitability from '../../components/Profitability';
import { extractError } from '../../api/client';

const { Title, Text } = Typography;

interface Props {
  versionId: number;
  projectId: number;
  duration: number;
  startDate: string;
}

export default function CalcResults({ versionId, projectId, duration, startDate }: Props) {
  const qc = useQueryClient();

  const { data: result, isLoading, refetch } = useQuery({
    queryKey: ['calc-result', versionId],
    queryFn: () => budgetsApi.calculate(versionId),
    enabled: false, // запускаем вручную
  });

  const calcMutation = useMutation({
    mutationFn: () => budgetsApi.calculate(versionId),
    onSuccess: (data) => {
      qc.setQueryData(['calc-result', versionId], data);
      qc.invalidateQueries({ queryKey: ['budget-version', versionId] });
      qc.invalidateQueries({ queryKey: ['budget-versions', projectId] });
      message.success('Расчёт выполнен');
    },
    onError: (e) => message.error(extractError(e)),
  });

  const r = result ?? calcMutation.data;

  const summaryRows = r ? [
    { label: 'ФОТ (вкл. взносы и НДФЛ)', value: fmtMoney(r.total_fot) },
    { label: 'Итого расходы без НДС', value: fmtMoney(r.total_costs) },
    { label: 'Итого стоимость работ (без НДС)', value: fmtMoney(r.total_revenue), highlight: true },
    { label: 'Выручка с НДС', value: fmtMoney(r.total_revenue_with_vat) },
    { label: 'Операционная прибыль', value: fmtMoney(r.operating_profit) },
    { label: 'Налог на прибыль', value: fmtMoney(r.tax) },
    { label: 'Чистая прибыль', value: fmtMoney(r.net_profit), highlight: true },
    {
      label: 'Рентабельность',
      value: <Profitability value={r.profitability} />,
      highlight: true,
    },
  ] : [];

  const monthlyColumns: ColumnsType<MonthlyResult> = [
    {
      title: 'Месяц',
      dataIndex: 'month',
      width: 90,
      render: (m: number) => monthLabel(startDate, m - 1),
      fixed: 'left',
    },
    { title: 'ФОТ вкл. взносы', dataIndex: 'total_fot', render: fmtMoney, align: 'right' },
    { title: 'Накладные', dataIndex: 'project_costs_ex_fot', render: fmtMoney, align: 'right' },
    { title: 'Непредвиденные', dataIndex: 'unpredictables', render: fmtMoney, align: 'right' },
    { title: 'АУП', dataIndex: 'aup', render: fmtMoney, align: 'right' },
    { title: 'БГ всего', render: (_, r) => fmtMoney(r.bg_execution + r.bg_warranty + r.bg_advance), align: 'right' },
    { title: 'Итого расходы', dataIndex: 'total_costs', render: fmtMoney, align: 'right' },
    { title: 'Выручка', dataIndex: 'revenue', render: fmtMoney, align: 'right' },
  ];

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button
          type="primary"
          icon={<CalculatorOutlined />}
          loading={calcMutation.isPending}
          onClick={() => calcMutation.mutate()}
          style={{ background: '#1a3a6b' }}
          size="large"
        >
          Рассчитать бюджет
        </Button>
        {r && (
          <Button
            icon={<FileExcelOutlined />}
            onClick={() => message.info('XLSX-выгрузка будет доступна в Этапе 5')}
          >
            Выгрузить XLSX
          </Button>
        )}
      </Space>

      {calcMutation.isPending && (
        <div style={{ textAlign: 'center', padding: 48 }}>
          <Spin size="large" tip="Выполняется расчёт..." />
        </div>
      )}

      {r && !calcMutation.isPending && (
        <>
          {/* Итоговые показатели */}
          <Row gutter={16} style={{ marginBottom: 24 }}>
            <Col span={6}>
              <Card>
                <Statistic
                  title="Стоимость работ без НДС (G236)"
                  value={r.total_revenue}
                  formatter={(v) => `${fmtNum(Number(v))} ₽`}
                  valueStyle={{ color: '#1a3a6b', fontWeight: 700 }}
                />
              </Card>
            </Col>
            <Col span={6}>
              <Card>
                <Statistic
                  title="Рентабельность (G244)"
                  value={r.profitability}
                  precision={2}
                  suffix="%"
                  // Цвет — по единой шкале, той же, что в реестре проектов.
                  valueStyle={{
                    color: profitabilityGrade(r.profitability).color,
                    fontWeight: 700,
                  }}
                />
              </Card>
            </Col>
            <Col span={6}>
              <Card>
                <Statistic
                  title="Чистая прибыль"
                  value={r.net_profit}
                  formatter={(v) => `${fmtNum(Number(v))} ₽`}
                  valueStyle={{ color: r.net_profit >= 0 ? '#52c41a' : '#ff4d4f' }}
                />
              </Card>
            </Col>
            <Col span={6}>
              <Card>
                <Statistic
                  title="ФОТ (вкл. взносы)"
                  value={r.total_fot}
                  formatter={(v) => `${fmtNum(Number(v))} ₽`}
                />
              </Card>
            </Col>
          </Row>

          {/* Сводная таблица */}
          <Card title="Итоги расчёта" size="small" style={{ marginBottom: 24 }}>
            <table style={{ width: '100%', borderCollapse: 'collapse' }}>
              <tbody>
                {summaryRows.map((row, i) => (
                  <tr key={i} style={{ background: row.highlight ? '#f6ffed' : undefined, borderTop: '1px solid #f0f0f0' }}>
                    <td style={{ padding: '8px 12px', fontWeight: row.highlight ? 600 : 400 }}>{row.label}</td>
                    <td style={{ padding: '8px 12px', textAlign: 'right', fontWeight: row.highlight ? 700 : 400, fontSize: row.highlight ? 15 : 13 }}>
                      {row.value}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </Card>

          {/* Помесячная разбивка */}
          <Card title="Помесячная разбивка" size="small">
            <Table
              rowKey="month"
              columns={monthlyColumns}
              dataSource={r.monthly}
              size="small"
              pagination={false}
              scroll={{ x: 900 }}
              summary={(rows) => (
                <Table.Summary fixed>
                  <Table.Summary.Row style={{ background: '#f0f5ff', fontWeight: 600 }}>
                    <Table.Summary.Cell index={0}>ИТОГО</Table.Summary.Cell>
                    <Table.Summary.Cell index={1} align="right">{fmtMoney(r.total_fot)}</Table.Summary.Cell>
                    <Table.Summary.Cell index={2} align="right">
                      {fmtMoney(rows.reduce((s, row) => s + row.project_costs_ex_fot, 0))}
                    </Table.Summary.Cell>
                    <Table.Summary.Cell index={3} align="right">
                      {fmtMoney(rows.reduce((s, row) => s + row.unpredictables, 0))}
                    </Table.Summary.Cell>
                    <Table.Summary.Cell index={4} align="right">
                      {fmtMoney(rows.reduce((s, row) => s + row.aup, 0))}
                    </Table.Summary.Cell>
                    <Table.Summary.Cell index={5} align="right">
                      {fmtMoney(rows.reduce((s, row) => s + row.bg_execution + row.bg_warranty + row.bg_advance, 0))}
                    </Table.Summary.Cell>
                    <Table.Summary.Cell index={6} align="right">{fmtMoney(r.total_costs)}</Table.Summary.Cell>
                    <Table.Summary.Cell index={7} align="right">{fmtMoney(r.total_revenue)}</Table.Summary.Cell>
                  </Table.Summary.Row>
                </Table.Summary>
              )}
            />
          </Card>
        </>
      )}
    </div>
  );
}
