import React from 'react';
import {
  Card, Button, Table, Space,
  message, Spin,
} from 'antd';
import { CalculatorOutlined } from '@ant-design/icons';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import type { ColumnsType } from 'antd/es/table';
import { budgetsApi } from '../../api';
import type { MonthlyResult } from '../../types';
import { fmtMoney, monthLabel, fmtNum } from '../../utils/fmt';
import { profitabilityGrade } from '../../utils/profitability';
import Profitability from '../../components/Profitability';
import { extractError } from '../../api/client';
import { FONT_NUM, LINE, TEXT_SOFT, RADIUS_LG } from '../../theme';

interface Props {
  versionId: number;
  projectId: number;
  startDate: string;
  /**
   * Версию уже считали — в budget_versions лежат стоимость и
   * рентабельность. Тогда результаты показываем сразу при открытии шага.
   */
  calculated: boolean;
}

export default function CalcResults({
  versionId, projectId, startDate, calculated,
}: Props) {
  const qc = useQueryClient();

  /**
   * Уже посчитанная версия открывается сразу с результатами.
   *
   * Расчёт — чистая функция от сохранённых данных, поэтому «показать
   * сохранённый» и «посчитать заново» здесь одно и то же: сервер
   * пересчитает по тем же вводам и вернёт тот же результат.
   *
   * Запрос включаем ТОЛЬКО для посчитанной версии. У непосчитанной
   * автозапуск был бы не безобиден: расчёт кэширует стоимость и
   * рентабельность в budget_versions, и в реестре появились бы цифры
   * у версии, которую никто не считал.
   */
  const { data: result, isFetching } = useQuery({
    queryKey: ['calc-result', versionId],
    queryFn: () => budgetsApi.calculate(versionId),
    enabled: calculated,
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
    { label: 'Операционная маржинальность', value: fmtMoney(r.operating_margin) },
    { label: 'Операционная прибыль', value: fmtMoney(r.operating_profit) },
    { label: 'Налог на прибыль', value: fmtMoney(r.tax) },
    { label: 'Чистая прибыль', value: fmtMoney(r.net_profit), highlight: true },
    {
      label: 'Рентабельность',
      value: <Profitability value={r.profitability} />,
      highlight: true,
    },
    {
      // Строка 249. Справочная: ни во что не входит и ни на что не
      // влияет — экономисту нужно просто видеть эту величину.
      label: `Стоимость + ставка рефинансирования на 1–4 месяцы (${fmtNum(r.ref_rate_pct)} %/год)`,
      value: fmtMoney(r.ref_rate_amount),
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
    { title: 'ФОТ вкл. взносы', dataIndex: 'total_fot', render: fmtMoney, align: 'right', className: 'ibcon-num' },
    { title: 'Накладные', dataIndex: 'project_costs_ex_fot', render: fmtMoney, align: 'right', className: 'ibcon-num' },
    { title: 'Непредвиденные', dataIndex: 'unpredictables', render: fmtMoney, align: 'right', className: 'ibcon-num' },
    { title: 'АУП', dataIndex: 'aup', render: fmtMoney, align: 'right', className: 'ibcon-num' },
    { title: 'БГ всего', render: (_, r) => fmtMoney(r.bg_execution + r.bg_warranty + r.bg_advance), align: 'right', className: 'ibcon-num' },
    { title: 'Итого расходы', dataIndex: 'total_costs', render: fmtMoney, align: 'right', className: 'ibcon-num' },
    { title: 'Выручка', dataIndex: 'revenue', render: fmtMoney, align: 'right', className: 'ibcon-num' },
  ];

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        {/* Кнопка остаётся и когда результат уже на экране: после правок
            на предыдущих шагах ею запускают пересчёт явно.
            Выгрузки здесь нет: книга собирается из трёх листов сразу —
            «Бюджет», БДР и БДДС, — и живёт на шаге «БДР и БДДС». */}
        <Button
          type="primary"
          icon={<CalculatorOutlined />}
          loading={calcMutation.isPending}
          onClick={() => calcMutation.mutate()}
          size="large"
        >
          {r ? 'Пересчитать бюджет' : 'Рассчитать бюджет'}
        </Button>
      </Space>

      {(calcMutation.isPending || (isFetching && !r)) && (
        <div style={{ textAlign: 'center', padding: 48 }}>
          <Spin size="large" />
        </div>
      )}

      {r && !calcMutation.isPending && (
        <>
          {/* Итоговые показатели — всегда одной строкой.
              Сетка Row/Col ломала их в столбик на узком экране, а
              показатели читают вместе: рентабельность рядом со
              стоимостью. Колонки сжимаются до минимума, а если и его не
              хватает — строка прокручивается вбок. */}
          <div style={{
            display: 'flex',
            flexWrap: 'nowrap',
            gap: 16,
            marginBottom: 24,
            overflowX: 'auto',
            paddingBottom: 4,
          }}>
            {[
              {
                title: 'Стоимость работ без НДС (G236)',
                value: `${fmtNum(r.total_revenue)} ₽`,
                color: 'var(--ibcon-brand)',
                bold: true,
              },
              {
                title: 'Рентабельность (G244)',
                value: `${fmtNum(r.profitability)} %`,
                // Цвет — по единой шкале, той же, что в реестре проектов.
                color: profitabilityGrade(r.profitability).color,
                bold: true,
              },
              {
                title: 'Чистая прибыль',
                value: `${fmtNum(r.net_profit)} ₽`,
                // Тот же цвет, что у рентабельности: это одна и та же
                // оценка бюджета, и разные цвета рядом читались как
                // разные оценки — «прибыль зелёная, рентабельность
                // красная». Своя шкала «плюс/минус» тут лишняя.
                color: profitabilityGrade(r.profitability).color,
                bold: false,
              },
              {
                title: 'ФОТ (вкл. взносы)',
                value: `${fmtNum(r.total_fot)} ₽`,
                color: undefined,
                bold: false,
              },
            ].map((s) => (
              <div
                key={s.title}
                style={{
                  flex: '1 1 0',
                  minWidth: 180,
                  padding: '14px 16px',
                  // Та же линия, что разлиновывает таблицы.
                  border: `1px solid ${LINE}`,
                  borderRadius: RADIUS_LG,
                }}
              >
                <div style={{
                  fontSize: 12,
                  color: TEXT_SOFT,
                  marginBottom: 6,
                  whiteSpace: 'nowrap',
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                }}>
                  {s.title}
                </div>
                <div style={{
                  fontSize: 20,
                  fontWeight: s.bold ? 700 : 500,
                  color: s.color,
                  fontFamily: FONT_NUM,
                  whiteSpace: 'nowrap',
                }}>
                  {s.value}
                </div>
              </div>
            ))}
          </div>

          {/* Сводная таблица */}
          <Card title="Итоги расчёта" size="small" style={{ marginBottom: 24 }}>
            <table style={{ width: '100%', borderCollapse: 'collapse' }}>
              <tbody>
                {summaryRows.map((row, i) => (
                  <tr key={i} style={{
                    background: row.highlight ? 'rgba(24, 62, 77, 0.04)' : undefined,
                    borderTop: `1px solid ${LINE}`,
                  }}>
                    <td style={{ padding: '8px 12px', fontWeight: row.highlight ? 600 : 400 }}>{row.label}</td>
                    <td style={{
                      padding: '8px 12px',
                      textAlign: 'right',
                      fontWeight: row.highlight ? 700 : 400,
                      fontSize: row.highlight ? 15 : 13,
                      fontFamily: FONT_NUM,
                    }}>
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
                  <Table.Summary.Row style={{ background: 'rgba(24, 62, 77, 0.05)', fontWeight: 600 }}>
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
