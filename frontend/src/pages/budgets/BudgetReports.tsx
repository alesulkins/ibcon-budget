import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Button, InputNumber, Segmented, Space, Spin, Switch, Table, Tooltip, Typography,
  message, Grid,
} from 'antd';
import { FileExcelOutlined, LeftOutlined, RightOutlined } from '@ant-design/icons';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import type { ColumnsType } from 'antd/es/table';
import { budgetsApi } from '../../api';
import type { BudgetReport, ReportRow } from '../../types';
import { extractError } from '../../api/client';
import { fmtNum, thousandFormatter, thousandParser } from '../../utils/fmt';
import { LINE, TEXT_SOFT } from '../../theme';
import { canIn, PERM } from '../../store/permissions';
import { useAutosave } from '../../hooks/useAutosave';
import { SCROLL_ROOT_ID } from '../../hooks/useScrollRestore';

interface Props {
  versionId: number;
  /** Права текущего пользователя на бюджеты этого проекта. */
  permissions?: string[];
  readonly?: boolean;
}

type Kind = 'bdr' | 'bdds';

/**
 * Ручные суммы статей: код статьи → значения по месяцам. Хранятся
 * отдельно для каждого отчёта, потому что одна и та же статья может быть
 * расчётной в БДР и ручной в БДДС.
 */
type ManualValues = Record<Kind, Record<string, number[]>>;

const EMPTY_MANUAL: ManualValues = { bdr: {}, bdds: {} };

/** Ключ ввода, под которым лежат ручные суммы отчётов. */
const MANUAL_INPUT = 'report_manual';

/**
 * БДР и БДДС. Оба отчёта приходят одним запросом: это два представления
 * одного расчёта, и раздельные запросы дали бы два разных расчёта.
 *
 * В кодификаторе больше сотни статей, а заполнены обычно единицы, поэтому
 * пустые строки по умолчанию скрыты — иначе отчёт приходится проматывать
 * целиком, чтобы найти три заполненные позиции. Ручные статьи при этом
 * показываются всегда: их не заполнить, если не видно.
 */
export default function BudgetReports({ versionId, permissions, readonly }: Props) {
  const qc = useQueryClient();
  const [kind, setKind] = useState<Kind>('bdr');
  const [showEmpty, setShowEmpty] = useState(false);
  const [manual, setManual] = useState<ManualValues>(EMPTY_MANUAL);
  // Телефон — до 768 точек (antd md), тот же порог, что и в каркасе.
  const mobile = !Grid.useBreakpoint().md;
  /**
   * Свёрнутая колонка статей. В отчёте больше сотни строк с длинными
   * названиями, и при сравнении месяцев между собой название мешает:
   * его сворачивают, оставляя узкий столбец с подсказкой по наведению.
   */
  const [foldNames, setFoldNames] = useState(false);
  /**
   * Обёртка таблицы и отступ, на котором стоит стрелка сворачивания.
   *
   * Стрелка живёт не в шапке: шапка уезжает вверх при прокрутке, а
   * свернуть колонку нужно в любой момент. Она прилеплена к правой
   * границе закреплённых колонок и скользит вместе с человеком по
   * вертикали; ширину этих колонок берём измерением — она зависит и от
   * ширины экрана, и от того, свёрнуто ли уже.
   */
  const [wrap, setWrap] = useState<HTMLDivElement | null>(null);
  const [toggleX, setToggleX] = useState(0);
  const [hydrated, setHydrated] = useState(false);

  useEffect(() => {
    if (!wrap) return;
    const measure = () => {
      const cells = wrap.querySelectorAll<HTMLElement>(
        '.ant-table-thead th.ant-table-cell-fix-start',
      );
      const last = cells[cells.length - 1];
      // Свёрнуто — закреплённых колонок нет вовсе, стрелка встаёт у
      // левого края таблицы.
      setToggleX(last
        ? last.getBoundingClientRect().right - wrap.getBoundingClientRect().left
        : 0);
    };
    measure();
    const ro = new ResizeObserver(measure);
    ro.observe(wrap);
    return () => ro.disconnect();
  }, [wrap, foldNames]);

  const { data, isLoading, error } = useQuery({
    queryKey: ['budget-reports', versionId],
    queryFn: () => budgetsApi.reports(versionId),
  });

  const { data: savedManual, isSuccess } = useQuery({
    queryKey: ['budget-input', versionId, MANUAL_INPUT],
    queryFn: () => budgetsApi.getInput<Partial<ManualValues>>(versionId, MANUAL_INPUT),
  });

  useEffect(() => {
    if (!isSuccess) return;
    setManual({
      bdr: savedManual?.bdr ?? {},
      bdds: savedManual?.bdds ?? {},
    });
    setHydrated(true);
  }, [savedManual, isSuccess]);

  const save = useCallback(
    (d: ManualValues) => budgetsApi.saveInput(versionId, MANUAL_INPUT, d),
    [versionId],
  );
  useAutosave({ data: manual, ready: hydrated, save, enabled: !readonly });

  const exportMutation = useMutation({
    mutationFn: () => budgetsApi.exportXlsx(versionId),
    onSuccess: (name) => message.success(`Файл «${name}» выгружен`),
    onError: (e) => message.error(extractError(e)),
  });

  const report: BudgetReport | undefined = data?.[kind];

  /**
   * Ручное значение уходит в отчёт не сразу: пересчёт запрашивается у
   * сервера после сохранения. До этого ячейка показывает введённое
   * число, а групповые суммы — прежние; ждать ответа на каждую цифру
   * было бы хуже, чем на секунду разошедшийся итог.
   */
  function setManualValue(code: string, monthIdx: number, value: number | null) {
    setManual(prev => {
      const forKind = { ...prev[kind] };
      const months = [...(forKind[code] ?? [])];
      while (months.length < (report?.months ?? 0)) months.push(0);
      months[monthIdx] = value ?? 0;
      // Строка из одних нулей не хранится: иначе ручных статей в JSON
      // накопится сотня, и все пустые.
      if (months.every(v => v === 0)) {
        delete forKind[code];
      } else {
        forKind[code] = months;
      }
      return { ...prev, [kind]: forKind };
    });
  }

  const manualFor = (code: string, monthIdx: number) =>
    manual[kind][code]?.[monthIdx] ?? 0;

  const rows = useMemo(() => {
    if (!report) return [];
    if (showEmpty) return report.rows;
    // Ручные статьи не прячем даже пустыми: иначе их нечем заполнить.
    return report.rows.filter(r => r.total !== 0 || r.manual);
  }, [report, showEmpty]);

  const canEdit = !readonly && canIn(permissions, PERM.budgetEdit);

  const columns: ColumnsType<ReportRow> = useMemo(() => {
    if (!report) return [];
    return [
      // Свёрнутое состояние прячет ЛЕВЫЙ край целиком — и кодификатор, и
      // статью: колонка в 44 точки всё равно занимала место, а месяцы
      // между собой сравнивают именно без неё.
      ...(foldNames ? [] : [
        // Кодификатор на телефоне не показываем: две закреплённые колонки
        // съедали почти всю ширину экрана, и месяцев было не видно. Номер
        // статьи есть в выгрузке, а на экране статью узнают по названию.
        ...(mobile ? [] : [{
          title: 'Кодификатор',
          dataIndex: 'code',
          width: 110,
          fixed: 'left' as const,
          className: 'ibcon-num',
          render: (v: string, r: ReportRow) => (
            <span style={{ color: TEXT_SOFT, fontWeight: r.group ? 600 : 400 }}>{v}</span>
          ),
        }]),
        {
          title: 'Статья оборотов',
          dataIndex: 'name',
          // На телефоне колонка уже и переносится по словам: закреплённая
          // колонка в 320 точек не оставила бы места месяцам.
          width: mobile ? 150 : 320,
          fixed: 'left' as const,
          render: (v: string, r: ReportRow) => (
            // Уровень показываем отступом: иерархия в кодификаторе, а не
            // в структуре данных — список статей плоский, как в форме.
            <span style={{
              paddingLeft: r.level * (mobile ? 8 : 14),
              fontWeight: r.group ? 600 : 400,
              whiteSpace: mobile ? 'normal' : undefined,
              display: 'inline-block',
            }}>
              {v}
            </span>
          ),
        },
      ]),
      ...report.month_labels.map((label, i) => ({
        title: label,
        key: `m${i}`,
        width: 130,
        align: 'right' as const,
        className: 'ibcon-num',
        render: (_: unknown, r: ReportRow) => {
          if (r.manual && canEdit) {
            return (
              <InputNumber
                size="small"
                style={{ width: '100%' }}
                value={manualFor(r.code, i) || null}
                placeholder="—"
                formatter={thousandFormatter}
                parser={thousandParser}
                onChange={(v) => setManualValue(r.code, i, v)}
              />
            );
          }
          return cellValue(r.monthly[i], r.group);
        },
      })),
      {
        title: 'Итого',
        dataIndex: 'total',
        width: 140,
        align: 'right' as const,
        // На телефоне итог не закрепляем: закреплённая слева статья и
        // закреплённый справа итог вдвоём съедали почти всю ширину
        // экрана — месяцев оставалось на полстолбца.
        fixed: mobile ? undefined : 'right',
        className: 'ibcon-num',
        render: (v: number, r: ReportRow) => cellValue(v, r.group, true),
      },
    ];
    // manual входит в зависимости: без него ячейки ввода не
    // перерисовывались бы при наборе.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [report, canEdit, manual, kind, mobile, foldNames]);

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
          onChange={(v) => setKind(v as Kind)}
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
        <Button
          onClick={() => qc.invalidateQueries({ queryKey: ['budget-reports', versionId] })}
        >
          Пересчитать отчёты
        </Button>
        {canIn(permissions, PERM.budgetExport) && (
          <Button
            type="primary"
            icon={<FileExcelOutlined />}
            loading={exportMutation.isPending}
            onClick={() => exportMutation.mutate()}
            style={{ marginLeft: 'auto' }}
          >
            Выгрузить книгу (Бюджет, БДР, БДДС)
          </Button>
        )}
      </Space>

      <Typography.Paragraph type="secondary" style={{ fontSize: 12, marginBottom: 8 }}>
        {kind === 'bdr'
          ? 'Отчёт о начислениях: суммы стоят в тех месяцах, в которых возникли.'
          : 'Отчёт о деньгах: зарплата и НДФЛ платятся двумя частями, взносы — '
            + 'в следующем месяце, налог на прибыль — на квартал позже, чем в БДР.'}
        {' '}Горизонт — вся длительность проекта.
        {canEdit && ' Статьи с полями ввода платформа не считает — их заполняют здесь; '
          + 'после ввода нажмите «Пересчитать отчёты», чтобы обновились групповые суммы.'}
      </Typography.Paragraph>

      {/**
        * Своей вертикальной прокрутки у таблицы нет ни на телефоне, ни на
        * широком экране: сначала листается страница — уезжают
        * переключатель и пояснение, — а дальше вниз идёт сама таблица.
        * Отчёт в узком окне посреди экрана читать нельзя, а боковая
        * прокрутка при этом одна на все строки сразу.
        *
        * Шапку держим липкой к странице, иначе к середине отчёта
        * непонятно, какой месяц перед глазами. Контейнер указываем явно:
        * страница прокручивается не в окне, а в #ibcon-scroll-root.
        */}
      <div ref={setWrap} style={{ position: 'relative' }}>
        {/* Стрелка сворачивания. Нулевой высоты и липкая: остаётся на
            виду, пока таблица на экране, и исчезает вместе с ней. По
            горизонтали стоит на границе закреплённых колонок — та при
            боковой прокрутке не двигается, поэтому стрелка всегда рядом
            со своей границей.

            Стоит ПЕРЕД таблицей: липкий элемент не поднимается выше
            своего места в потоке, и снизу он прилипал бы только у самого
            конца отчёта. */}
        {rows.length > 0 && (
          <div style={{
            position: 'sticky',
            top: '45vh',
            height: 0,
            // Выше закреплённых колонок и липкой полосы прокрутки: они
            // перехватывали нажатие на стрелку.
            zIndex: 20,
            pointerEvents: 'none',
          }}>
            <Tooltip title={foldNames ? 'Показать статьи' : 'Скрыть статьи'} placement="right">
              <span
                role="button"
                aria-label={foldNames ? 'Показать статьи' : 'Скрыть статьи'}
                className="ibcon-fold-toggle"
                onClick={() => setFoldNames(v => !v)}
                style={{ left: toggleX, pointerEvents: 'auto' }}
              >
                {foldNames ? <RightOutlined /> : <LeftOutlined />}
              </span>
            </Tooltip>
          </div>
        )}
        <Table
          rowKey="code"
          columns={columns}
          dataSource={rows}
          size="small"
          pagination={false}
          tableLayout="fixed"
          sticky={{ getContainer: () => document.getElementById(SCROLL_ROOT_ID) ?? window }}
          scroll={{ x: 'max-content' }}
          rowClassName={(r) => (r.group ? 'ibcon-report-group' : '')}
          locale={{ emptyText: 'Нет заполненных статей — версию ещё не считали.' }}
        />

      </div>
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
