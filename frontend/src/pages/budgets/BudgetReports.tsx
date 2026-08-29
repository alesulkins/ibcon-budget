import { useMemo, useState } from 'react';
import { Button, Segmented, Space, Spin, Switch, Table, Typography, message } from 'antd';
import { FileExcelOutlined } from '@ant-design/icons';
import { useQuery, useMutation } from '@tanstack/react-query';
import type { ColumnsType } from 'antd/es/table';
import { budgetsApi } from '../../api';
import type { BudgetReport, ReportRow } from '../../types';
import { extractError } from '../../api/client';
import { fmtNum } from '../../utils/fmt';
import { LINE, TEXT_SOFT } from '../../theme';
import { canIn, PERM } from '../../store/permissions';

interface Props {
  versionId: number;
  /** Права текущего пользователя на бюджеты этого проекта. */
  permissions?: string[];
}

/**
 * БДР и БДДС. Оба отчёта приходят одним запросом: это два представления
 * одного расчёта, и раздельные запросы дали бы два разных расчёта.
 *
 * В кодификаторе больше сотни статей, а заполнены обычно единицы, поэтому
 * пустые строки по умолчанию скрыты — иначе отчёт приходится
 * проматывать целиком, чтобы найти три заполненные позиции.
 */
export default function BudgetReports({ versionId, permissions }: Props) {
  const [kind, setKind] = useState<'bdr' | 'bdds'>('bdr');
  const [showEmpty, setShowEmpty] = useState(false);

  const { data, isLoading, error } = useQuery({
    queryKey: ['budget-reports', versionId],
    queryFn: () => budgetsApi.reports(versionId),
  });

  const exportMutation = useMutation({
    mutationFn: () => budgetsApi.exportReports(versionId),
    onSuccess: (name) => message.success(`Файл «${name}» выгружен`),
    onError: (e) => message.error(extractError(e)),
  });

  const report: BudgetReport | undefined = data?.[kind];

  /**
   * Пустая группа скрывается вместе со своими статьями: если внутри
   * «Содержания транспорта» ничего нет, сама группа тоже лишняя.
   * Групповая строка уже содержит сумму вложенных, поэтому достаточно
   * посмотреть на её итог.
   */
  const rows = useMemo(() => {
    if (!report) return [];
    if (showEmpty) return report.rows;
    return report.rows.filter(r => r.total !== 0);
  }, [report, showEmpty]);

  const columns: ColumnsType<ReportRow> = useMemo(() => {
    if (!report) return [];
    return [
      {
        title: 'Код',
        dataIndex: 'code',
        width: 90,
        fixed: 'left',
        className: 'ibcon-num',
        render: (v: string, r) => (
          <span style={{ color: TEXT_SOFT, fontWeight: r.group ? 600 : 400 }}>{v}</span>
        ),
      },
      {
        title: 'Статья оборотов',
        dataIndex: 'name',
        width: 320,
        fixed: 'left',
        render: (v: string, r) => (
          // Уровень показываем отступом: иерархия в кодификаторе, а не в
          // структуре данных — список статей плоский, как в форме.
          <span style={{ paddingLeft: r.level * 14, fontWeight: r.group ? 600 : 400 }}>
            {v}
          </span>
        ),
      },
      ...report.month_labels.map((label, i) => ({
        title: label,
        key: `m${i}`,
        width: 120,
        align: 'right' as const,
        className: 'ibcon-num',
        render: (_: unknown, r: ReportRow) => cellValue(r.monthly[i], r.group),
      })),
      {
        title: 'Итого',
        dataIndex: 'total',
        width: 140,
        align: 'right' as const,
        fixed: 'right',
        className: 'ibcon-num',
        render: (v: number, r: ReportRow) => cellValue(v, r.group, true),
      },
    ];
  }, [report]);

  if (isLoading) {
    return <div style={{ textAlign: 'center', padding: 48 }}><Spin size="large" /></div>;
  }
  if (error) {
    return <Typography.Text type="danger">{extractError(error)}</Typography.Text>;
  }

  return (
    <div>
      <Space style={{ marginBottom: 16 }} wrap>
        <Segmented
          value={kind}
          onChange={(v) => setKind(v as 'bdr' | 'bdds')}
          options={[
            { value: 'bdr', label: 'БДР' },
            { value: 'bdds', label: 'БДДС' },
          ]}
        />
        <Space size={6}>
          <Switch size="small" checked={showEmpty} onChange={setShowEmpty} />
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            Показывать незаполненные статьи
          </Typography.Text>
        </Space>
        {canIn(permissions, PERM.budgetExport) && (
          <Button
            icon={<FileExcelOutlined />}
            loading={exportMutation.isPending}
            onClick={() => exportMutation.mutate()}
            style={{ marginLeft: 'auto' }}
          >
            Выгрузить БДР и БДДС
          </Button>
        )}
      </Space>

      <Typography.Paragraph type="secondary" style={{ fontSize: 12 }}>
        {kind === 'bdr'
          ? 'Отчёт о начислениях: суммы стоят в тех месяцах, в которых возникли.'
          : 'Отчёт о деньгах: зарплата и НДФЛ платятся двумя частями, взносы — '
            + 'в следующем месяце, налог на прибыль — на квартал позже, чем в БДР.'}
        {' '}Горизонт — вся длительность проекта.
      </Typography.Paragraph>

      <Table
        rowKey="code"
        columns={columns}
        dataSource={rows}
        size="small"
        pagination={false}
        tableLayout="fixed"
        scroll={{ x: 'max-content', y: 560 }}
        rowClassName={(r) => (r.group ? 'ibcon-report-group' : '')}
        locale={{ emptyText: 'Нет заполненных статей — версию ещё не считали.' }}
      />
    </div>
  );
}

/** Ноль показываем прочерком: в отчёте сотня статей, и почти все пустые. */
function cellValue(v: number, group: boolean, total = false) {
  if (v === 0) return <span style={{ color: LINE }}>—</span>;
  return (
    <span style={{ fontWeight: group || total ? 600 : 400 }}>{fmtNum(v)}</span>
  );
}
