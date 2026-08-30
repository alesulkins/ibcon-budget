import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  Tabs, Table, Button, Modal, Form, Input, InputNumber, Select, Switch, Grid,
  Space, Tooltip, Typography, message,
} from 'antd';
import { PlusOutlined, EditOutlined, LockOutlined } from '@ant-design/icons';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import type { ColumnsType } from 'antd/es/table';
import { refsApi } from '../../api';
import type { Executor, Position, WorkMode, CostItem, CitySalary } from '../../types';
import { PERM, usePermissions } from '../../store/permissions';
import { extractError } from '../../api/client';
import { fmtDateTime, fmtNum, thousandFormatter, thousandParser } from '../../utils/fmt';
import { shortName } from '../../utils/names';
import StatusTag from '../../components/StatusTag';
import { TEXT_SOFT } from '../../theme';
import CitySalaries from './CitySalaries';
import { useFillToSiderFooter } from '../../hooks/useFillHeight';

const { Text } = Typography;

/**
 * Правка справочников — по праву references.edit. Базово оно есть
 * только у главного экономиста, но он может выдать его отдельно.
 */
const useCanEdit = () => usePermissions().can(PERM.referencesEdit);

/**
 * Кнопка «Добавить …» вынесена в панель вкладок, чтобы таблица начиналась
 * на той же высоте, что и в реестре проектов. Нажатие приходит во вкладку
 * счётчиком: активная вкладка одна (destroyOnHidden), поэтому сигнал
 * получает ровно та, что на экране.
 */
interface TabProps { addSignal: number }

function useAddSignal(addSignal: number, open: () => void) {
  // Значение на момент монтирования пропускаем: при переключении вкладки
  // счётчик уже не нулевой, и форма открывалась бы сама собой.
  const seen = useRef(addSignal);
  useEffect(() => {
    if (addSignal === seen.current) return;
    seen.current = addSignal;
    open();
  }, [addSignal, open]);
}

// ─── Общее для всех вкладок ─────────────────────────────────────────────────

/**
 * «Ничего не выбрано» — это undefined, а не значение 'all'. Так фильтр
 * показывает подпись плейсхолдером, а он у antd серый — того же тона, что
 * рамка списка. Со значением-заглушкой текст рисовался бы чёрным, будто
 * фильтр включён.
 */
type StatusFilter = 'active' | 'inactive';

const STATUS_OPTIONS = [
  { value: 'active', label: 'Только активные' },
  { value: 'inactive', label: 'Только неактивные' },
];

/**
 * Поиск по названию и фильтр по активности — одинаковые во всех
 * справочниках (общее требование к справочникам). Сортировка по алфавиту
 * приходит с бэкенда, колонки её дублируют как переключатель.
 *
 * `extra` — дополнительные фильтры конкретной вкладки; встают справа от
 * общих, чтобы строка фильтров везде начиналась одинаково.
 */
function useRefFilter<T extends { active: boolean }>(
  rows: T[] | undefined,
  text: (row: T) => string,
  extra?: React.ReactNode,
) {
  const [search, setSearch] = useState('');
  const [status, setStatus] = useState<StatusFilter>();

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    return (rows ?? [])
      .filter(r => {
        if (status === 'active' && !r.active) return false;
        if (status === 'inactive' && r.active) return false;
        return !q || text(r).toLowerCase().includes(q);
      })
      // Неактивные — в конец списка при любой сортировке колонок:
      // работают всегда с активными записями, деактивированные нужны
      // только чтобы их увидеть.
      .sort((a, b) => (a.active === b.active ? 0 : a.active ? -1 : 1));
    // text — стабильная функция сравнения строки, пересоздаётся каждый
    // рендер; в зависимости её не берём, иначе фильтр считался бы заново
    // на каждый рендер вкладки.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rows, search, status]);

  const controls = (
    // wrap — на узком экране поиск и фильтр переносятся на свои строки,
    // а не сжимаются до нечитаемого.
    <Space style={{ marginBottom: 12 }} wrap>
      <Input.Search
        allowClear
        placeholder="Поиск по названию"
        style={{ width: 280, maxWidth: '100%' }}
        value={search}
        onChange={(e) => setSearch(e.target.value)}
      />
      <Select
        allowClear
        placeholder="Все статусы"
        style={{ width: 180 }}
        value={status}
        onChange={(v) => setStatus(v as StatusFilter)}
        options={STATUS_OPTIONS}
      />
      {extra}
    </Space>
  );

  return { filtered, controls };
}

/**
 * Неактивные записи должны оставаться в конце и после клика по сортировке
 * колонки — иначе деактивированная строка всплывала бы наверх из-за
 * названия. Обёртка вокруг сравнения самой колонки.
 */
function activeFirst<T extends { active: boolean }>(cmp: (a: T, b: T) => number) {
  return (a: T, b: T) => (a.active !== b.active ? (a.active ? -1 : 1) : cmp(a, b));
}

function statusColumn<T extends { active: boolean }>(
  labels: [string, string] = ['Активна', 'Неактивна'],
): ColumnsType<T>[number] {
  return {
    title: 'Статус',
    dataIndex: 'active',
    width: 130,
    render: (v: boolean) => (
      <StatusTag color={v ? 'green' : 'grey'}>{v ? labels[0] : labels[1]}</StatusTag>
    ),
  };
}

/** Кто и когда менял запись — общее требование к справочникам. */
function editorColumn<T extends { updated_at: string; updated_by_name?: string }>(): ColumnsType<T>[number] {
  return {
    title: 'Изменено',
    key: 'updated',
    width: 220,
    render: (_: unknown, r: T) => (
      <Text type="secondary" style={{ fontSize: 12 }}>
        {fmtDateTime(r.updated_at)}
        {r.updated_by_name ? ` · ${shortName(r.updated_by_name)}` : ''}
      </Text>
    ),
  };
}

const TABLE_PROPS = {
  rowKey: 'id' as const,
  className: 'nowrap-table',
  size: 'small' as const,
  pagination: false as const,
};

/**
 * Таблица справочника прокручивается сама и не заходит за линию ЛК в
 * сайдбаре — как реестр проектов и история изменений. Должностей почти
 * полсотни, страницами их не листаем.
 */
function RefTable<T extends object>({ columns, dataSource, loading }: {
  columns: ColumnsType<T>;
  dataSource: T[];
  loading: boolean;
}) {
  const [fillRef, fillHeight] = useFillToSiderFooter<HTMLDivElement>();
  return (
    <div ref={fillRef}>
      <Table<T>
        {...TABLE_PROPS}
        columns={columns}
        dataSource={dataSource}
        loading={loading}
        scroll={{ x: 'max-content', y: fillHeight }}
      />
    </div>
  );
}

// ─── Исполнители ────────────────────────────────────────────────────────────
function ExecutorsTab({ addSignal }: TabProps) {
  const canEdit = useCanEdit();
  const qc = useQueryClient();
  const [showModal, setShowModal] = useState(false);
  const [editing, setEditing] = useState<Executor | null>(null);
  const [form] = Form.useForm();

  const openAdd = useCallback(() => {
    setEditing(null);
    form.resetFields();
    setShowModal(true);
  }, [form]);
  useAddSignal(addSignal, openAdd);

  const { data, isLoading } = useQuery({
    queryKey: ['executors-all'],
    queryFn: () => refsApi.executors(true),
  });
  const { filtered, controls } = useRefFilter(data, r => `${r.name} ${r.full_name}`);

  const invalidate = () => {
    qc.invalidateQueries({ queryKey: ['executors-all'] });
    qc.invalidateQueries({ queryKey: ['executors'] });
  };

  const createMutation = useMutation({
    mutationFn: (vals: Parameters<typeof refsApi.createExecutor>[0]) => refsApi.createExecutor(vals),
    onSuccess: () => {
      invalidate();
      message.success('Исполнитель добавлен');
      setShowModal(false);
      form.resetFields();
    },
    onError: (e) => message.error(extractError(e)),
  });

  const updateMutation = useMutation({
    mutationFn: (vals: Parameters<typeof refsApi.updateExecutor>[1]) =>
      refsApi.updateExecutor(editing!.id, vals),
    onSuccess: () => {
      invalidate();
      message.success('Обновлено');
      setShowModal(false);
      form.resetFields();
      setEditing(null);
    },
    onError: (e) => message.error(extractError(e)),
  });

  const columns: ColumnsType<Executor> = [
    { title: 'Наименование (кратко)', dataIndex: 'name', defaultSortOrder: 'ascend', sorter: activeFirst<Executor>((a, b) => a.name.localeCompare(b.name, 'ru')) },
    { title: 'Полное наименование', dataIndex: 'full_name' },
    statusColumn<Executor>(['Активен', 'Неактивен']),
    editorColumn<Executor>(),
    ...(canEdit ? [{
      // Колонок много, таблица шире экрана — кнопку правки закрепляем
      // справа, иначе до неё пришлось бы доскроллить.
      title: '', key: 'edit', width: 60, fixed: 'right' as const,
      render: (_: unknown, r: Executor) => (
        <Button size="small" icon={<EditOutlined />}
          onClick={() => { setEditing(r); form.setFieldsValue(r); setShowModal(true); }} />
      ),
    }] : []),
  ];

  return (
    <>
      {controls}
      <RefTable columns={columns} dataSource={filtered} loading={isLoading} />
      <Modal
        title={editing ? 'Редактирование исполнителя' : 'Новый исполнитель'}
        open={showModal}
        onCancel={() => { setShowModal(false); setEditing(null); form.resetFields(); }}
        onOk={() => form.submit()} okText="Сохранить" cancelText="Отмена"
        confirmLoading={createMutation.isPending || updateMutation.isPending}
      >
        <Form
          form={form}
          layout="vertical"
          initialValues={{ profit_tax_rate: 25, refinancing_rate: 14.5 }}
          onFinish={(vals) => editing ? updateMutation.mutate(vals) : createMutation.mutate(vals)}
        >
          <Form.Item name="name" label="Краткое наименование" rules={[{ required: true }]}>
            <Input placeholder="Айбикон" />
          </Form.Item>
          <Form.Item name="full_name" label="Полное наименование" rules={[{ required: !editing }]}>
            <Input placeholder='ООО «АйБиКон»' />
          </Form.Item>
          <Form.Item
            name="profit_tax_rate"
            label="Налог на прибыль, %"
            extra="Подставляется в параметры бюджета по умолчанию, экономист может изменить."
          >
            <InputNumber style={{ width: '100%' }} min={0} max={100} step={0.5} />
          </Form.Item>
          <Form.Item name="refinancing_rate" label="Ставка рефинансирования, %">
            <InputNumber style={{ width: '100%' }} min={0} max={100} step={0.5} />
          </Form.Item>
          {editing && (
            <Form.Item
              name="active"
              label="Активен"
              valuePropName="checked"
              extra="Деактивация убирает исполнителя из выпадающих списков. Уже созданные бюджеты не меняются."
            >
              <Switch />
            </Form.Item>
          )}
        </Form>
      </Modal>
    </>
  );
}

// ─── Должности ──────────────────────────────────────────────────────────────
function PositionsTab({ addSignal }: TabProps) {
  const canEdit = useCanEdit();
  const qc = useQueryClient();
  const [showModal, setShowModal] = useState(false);
  const [editing, setEditing] = useState<Position | null>(null);
  // Оклады по городам живут отдельно от формы: они не поле, а таблица, и
  // сохраняются своим запросом.
  const [citySalaries, setCitySalaries] = useState<CitySalary[]>([]);
  const [form] = Form.useForm();

  const openAdd = useCallback(() => {
    setCitySalaries([]);
    setEditing(null);
    form.resetFields();
    setShowModal(true);
  }, [form]);
  useAddSignal(addSignal, openAdd);

  const { data, isLoading } = useQuery({
    queryKey: ['positions-all'],
    queryFn: () => refsApi.positions(true),
  });
  const { filtered, controls } = useRefFilter(data, r => r.name);

  const invalidate = () => {
    qc.invalidateQueries({ queryKey: ['positions-all'] });
    qc.invalidateQueries({ queryKey: ['positions'] });
  };

  const createMutation = useMutation({
    mutationFn: async (vals: { name: string; salary?: number; is_itr?: boolean }) => {
      const p = await refsApi.createPosition(vals);
      if (citySalaries.length === 0) return p;
      return refsApi.setCitySalaries(p.id, citySalaries);
    },
    onSuccess: () => {
      invalidate();
      message.success('Должность добавлена');
      setShowModal(false);
      form.resetFields();
    },
    onError: (e) => message.error(extractError(e)),
  });

  const updateMutation = useMutation({
    mutationFn: async (vals: { name?: string; salary?: number; is_itr?: boolean; active?: boolean }) => {
      const p = await refsApi.updatePosition(editing!.id, vals);
      // Оклады по городам — отдельный запрос: они хранятся своей
      // таблицей, а не полем должности.
      return refsApi.setCitySalaries(p.id, citySalaries);
    },
    onSuccess: () => {
      invalidate();
      message.success('Обновлено');
      setShowModal(false);
      form.resetFields();
      setEditing(null);
    },
    onError: (e) => message.error(extractError(e)),
  });

  const columns: ColumnsType<Position> = [
    { title: 'Должность', dataIndex: 'name', defaultSortOrder: 'ascend', sorter: activeFirst<Position>((a, b) => a.name.localeCompare(b.name, 'ru')) },
    {
      title: 'Зарплата, ₽',
      dataIndex: 'salary',
      className: 'ibcon-num',
      width: 220,
      sorter: activeFirst<Position>((a, b) => a.salary - b.salary),
      render: (v: number, r: Position) => {
        const cities = r.city_salaries ?? [];
        if (cities.length === 0) return fmtNum(v);
        // Городские ставки важнее оклада по умолчанию: именно они
        // подставляются в мастер, если город проекта совпал.
        return (
          <Tooltip
            title={cities.map(cs => `${cs.city_name}: ${fmtNum(cs.salary)} ₽`).join('\n')}
          >
            <span>
              {fmtNum(v)}
              <span style={{ color: TEXT_SOFT, fontSize: 12 }}>
                {' '}· {cities.length} гор.
              </span>
            </span>
          </Tooltip>
        );
      },
    },
    {
      title: 'ИТР',
      dataIndex: 'is_itr',
      width: 90,
      render: (v: boolean) => (
        <StatusTag color={v ? 'teal' : 'grey'}>{v ? 'Да' : 'Нет'}</StatusTag>
      ),
    },
    statusColumn<Position>(),
    editorColumn<Position>(),
    ...(canEdit ? [{
      // Колонок много, таблица шире экрана — кнопку правки закрепляем
      // справа, иначе до неё пришлось бы доскроллить.
      title: '', key: 'edit', width: 60, fixed: 'right' as const,
      render: (_: unknown, r: Position) => (
        <Button size="small" icon={<EditOutlined />}
          onClick={() => {
            setEditing(r);
            setCitySalaries(r.city_salaries ?? []);
            form.setFieldsValue(r);
            setShowModal(true);
          }} />
      ),
    }] : []),
  ];

  return (
    <>
      {controls}
      <RefTable columns={columns} dataSource={filtered} loading={isLoading} />
      <Modal
        title={editing ? 'Редактирование должности' : 'Новая должность'}
        open={showModal}
        onCancel={() => { setShowModal(false); setEditing(null); form.resetFields(); }}
        onOk={() => form.submit()} okText="Сохранить" cancelText="Отмена"
        confirmLoading={createMutation.isPending || updateMutation.isPending}
      >
        <Form
          form={form}
          layout="vertical"
          initialValues={{ salary: 0 }}
          onFinish={(vals) => editing ? updateMutation.mutate(vals) : createMutation.mutate(vals)}
        >
          <Form.Item name="name" label="Наименование" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item
            name="salary"
            label="Зарплата, ₽"
            extra="Подставляется в шаг «Сотрудники» как значение по умолчанию, экономист может изменить его вручную."
          >
            <InputNumber
              style={{ width: '100%' }}
              min={0}
              formatter={thousandFormatter}
              parser={thousandParser}
            />
          </Form.Item>
          <Form.Item
            name="is_itr"
            label="ИТР"
            valuePropName="checked"
            extra="Инженерно-технический работник. По этому признаку считается сводка по ИТР в выгрузке бюджета."
          >
            <Switch checkedChildren="Да" unCheckedChildren="Нет" />
          </Form.Item>
          <Form.Item
            label="Зарплата по городам"
            extra="В разных городах за одну и ту же работу платят по-разному."
          >
            <CitySalaries value={citySalaries} onChange={setCitySalaries} />
          </Form.Item>
          {editing && (
            <Form.Item
              name="active"
              label="Активна"
              valuePropName="checked"
              extra="Деактивация убирает должность из выпадающих списков. Уже созданные бюджеты не меняются."
            >
              <Switch />
            </Form.Item>
          )}
        </Form>
      </Modal>
    </>
  );
}

// ─── Режимы работы ──────────────────────────────────────────────────────────
function WorkModesTab({ addSignal }: TabProps) {
  const canEdit = useCanEdit();
  const qc = useQueryClient();
  const [showModal, setShowModal] = useState(false);
  const [editing, setEditing] = useState<WorkMode | null>(null);
  const [form] = Form.useForm();

  const openAdd = useCallback(() => {
    setEditing(null);
    form.resetFields();
    setShowModal(true);
  }, [form]);
  useAddSignal(addSignal, openAdd);

  const { data, isLoading } = useQuery({
    queryKey: ['work-modes-all'],
    queryFn: () => refsApi.workModes(true),
  });
  const { filtered, controls } = useRefFilter(data, r => `${r.code} ${r.full_name}`);

  const invalidate = () => {
    qc.invalidateQueries({ queryKey: ['work-modes-all'] });
    qc.invalidateQueries({ queryKey: ['work-modes'] });
  };

  const createMutation = useMutation({
    mutationFn: (vals: { code: string; full_name: string }) => refsApi.createWorkMode(vals),
    onSuccess: () => {
      invalidate();
      message.success('Режим работы добавлен');
      setShowModal(false);
      form.resetFields();
    },
    onError: (e) => message.error(extractError(e)),
  });

  const updateMutation = useMutation({
    mutationFn: (vals: { full_name?: string; active?: boolean }) =>
      refsApi.updateWorkMode(editing!.id, vals),
    onSuccess: () => {
      invalidate();
      message.success('Обновлено');
      setShowModal(false);
      form.resetFields();
      setEditing(null);
    },
    onError: (e) => message.error(extractError(e)),
  });

  const columns: ColumnsType<WorkMode> = [
    { title: 'Код', dataIndex: 'code', width: 100, defaultSortOrder: 'ascend', sorter: activeFirst<WorkMode>((a, b) => a.code.localeCompare(b.code, 'ru')) },
    { title: 'Полное наименование', dataIndex: 'full_name' },
    statusColumn<WorkMode>(['Активен', 'Неактивен']),
    editorColumn<WorkMode>(),
    ...(canEdit ? [{
      // Колонок много, таблица шире экрана — кнопку правки закрепляем
      // справа, иначе до неё пришлось бы доскроллить.
      title: '', key: 'edit', width: 60, fixed: 'right' as const,
      render: (_: unknown, r: WorkMode) => (
        <Button size="small" icon={<EditOutlined />}
          onClick={() => { setEditing(r); form.setFieldsValue(r); setShowModal(true); }} />
      ),
    }] : []),
  ];

  return (
    <>
      {controls}
      <RefTable columns={columns} dataSource={filtered} loading={isLoading} />
      <Modal
        title={editing ? 'Редактирование' : 'Новый режим работы'}
        open={showModal}
        onCancel={() => { setShowModal(false); setEditing(null); form.resetFields(); }}
        onOk={() => form.submit()} okText="Сохранить" cancelText="Отмена"
        confirmLoading={createMutation.isPending || updateMutation.isPending}
      >
        <Form form={form} layout="vertical"
          onFinish={(vals) => editing ? updateMutation.mutate(vals) : createMutation.mutate(vals)}>
          {!editing && <Form.Item name="code" label="Код" rules={[{ required: true }]}><Input placeholder="4/2" /></Form.Item>}
          <Form.Item name="full_name" label="Полное наименование" rules={[{ required: true }]}>
            <Input placeholder="Описание режима работы" />
          </Form.Item>
          {editing && (
            <Form.Item name="active" label="Активен" valuePropName="checked"
              extra="Деактивация убирает режим из выпадающих списков. Уже созданные бюджеты не меняются.">
              <Switch />
            </Form.Item>
          )}
        </Form>
      </Modal>
    </>
  );
}

// ─── Статьи затрат ──────────────────────────────────────────────────────────
function CostItemsTab({ addSignal }: TabProps) {
  const canEdit = useCanEdit();
  const qc = useQueryClient();
  const [showModal, setShowModal] = useState(false);
  const [editing, setEditing] = useState<CostItem | null>(null);
  const [form] = Form.useForm();

  const openAdd = useCallback(() => {
    setEditing(null);
    form.resetFields();
    setShowModal(true);
  }, [form]);
  useAddSignal(addSignal, openAdd);

  const { data, isLoading } = useQuery({
    queryKey: ['cost-items-all'],
    queryFn: () => refsApi.costItems(true),
  });

  // Признак «расчётная» делит справочник надвое по смыслу: за одними
  // статьями стоит формула листа 4.x, у других сумму вводят руками.
  // Фильтр по нему нужен чаще, чем по статусу.
  const [calcFilter, setCalcFilter] = useState<'yes' | 'no'>();
  const { filtered, controls } = useRefFilter(
    data,
    r => r.name,
    <Select
      allowClear
      placeholder="Расчётная: все"
      style={{ width: 200 }}
      value={calcFilter}
      onChange={(v) => setCalcFilter(v)}
      options={[
        { value: 'yes', label: 'Расчётная: да' },
        { value: 'no', label: 'Расчётная: нет' },
      ]}
    />,
  );
  const rows = calcFilter === undefined
    ? filtered
    : filtered.filter(r => r.is_calculated === (calcFilter === 'yes'));

  const invalidate = () => {
    qc.invalidateQueries({ queryKey: ['cost-items-all'] });
    qc.invalidateQueries({ queryKey: ['cost-items'] });
  };

  const createMutation = useMutation({
    mutationFn: (vals: { name: string }) => refsApi.createCostItem(vals),
    onSuccess: () => {
      invalidate();
      message.success('Статья затрат добавлена');
      setShowModal(false);
      form.resetFields();
    },
    onError: (e) => message.error(extractError(e)),
  });

  const updateMutation = useMutation({
    mutationFn: (vals: { name?: string; active?: boolean }) =>
      refsApi.updateCostItem(editing!.id, vals),
    onSuccess: () => {
      invalidate();
      message.success('Обновлено');
      setShowModal(false);
      form.resetFields();
      setEditing(null);
    },
    onError: (e) => message.error(extractError(e)),
  });

  const columns: ColumnsType<CostItem> = [
    { title: 'Статья затрат', dataIndex: 'name', defaultSortOrder: 'ascend', sorter: activeFirst<CostItem>((a, b) => a.name.localeCompare(b.name, 'ru')) },
    {
      title: 'Расчётная',
      dataIndex: 'is_calculated',
      width: 130,
      render: (v: boolean) => (
        <StatusTag color={v ? 'violet' : 'grey'}>{v ? 'Да' : 'Нет'}</StatusTag>
      ),
    },
    statusColumn<CostItem>(),
    editorColumn<CostItem>(),
    ...(canEdit ? [{
      // Колонок много, таблица шире экрана — кнопку правки закрепляем
      // справа, иначе до неё пришлось бы доскроллить.
      title: '', key: 'edit', width: 60, fixed: 'right' as const,
      render: (_: unknown, r: CostItem) => r.is_calculated ? (
        // За расчётной статьёй стоит формула листа 4.x: её нельзя ни
        // переименовать, ни деактивировать. Бэкенд отказывает независимо.
        <Tooltip title="Расчётную статью изменить нельзя: её сумму даёт формула">
          <Button size="small" icon={<LockOutlined />} disabled />
        </Tooltip>
      ) : (
        <Button size="small" icon={<EditOutlined />}
          onClick={() => { setEditing(r); form.setFieldsValue(r); setShowModal(true); }} />
      ),
    }] : []),
  ];

  return (
    <>
      {controls}
      <RefTable columns={columns} dataSource={rows} loading={isLoading} />
      <Modal
        title={editing ? 'Редактирование статьи затрат' : 'Новая статья затрат'}
        open={showModal}
        onCancel={() => { setShowModal(false); setEditing(null); form.resetFields(); }}
        onOk={() => form.submit()} okText="Сохранить" cancelText="Отмена"
        confirmLoading={createMutation.isPending || updateMutation.isPending}
      >
        <Form form={form} layout="vertical"
          onFinish={(vals) => editing ? updateMutation.mutate(vals) : createMutation.mutate(vals)}>
          <Form.Item
            name="name"
            label="Название"
            rules={[{ required: true }]}
            extra="Добавляются только нерасчётные статьи — суммы по ним вводятся вручную."
          >
            <Input placeholder="Например: Аренда склада" />
          </Form.Item>
          {editing && (
            <Form.Item name="active" label="Активна" valuePropName="checked"
              extra="Деактивация убирает статью из выпадающих списков. Уже созданные бюджеты не меняются.">
              <Switch />
            </Form.Item>
          )}
        </Form>
      </Modal>
    </>
  );
}

export default function ReferencesPage() {
  const canEdit = useCanEdit();
  const mobile = !Grid.useBreakpoint().md;
  const [tab, setTab] = useState('executors');
  const [addSignal, setAddSignal] = useState(0);

  const ADD_LABELS: Record<string, string> = {
    executors: 'Добавить исполнителя',
    positions: 'Добавить должность',
    'work-modes': 'Добавить режим работы',
    'cost-items': 'Добавить статью затрат',
  };

  const addButton = canEdit && (
    <Button
      type="primary"
      icon={<PlusOutlined />}
      onClick={() => setAddSignal(s => s + 1)}
      // На телефоне кнопка стоит своей строкой во всю ширину.
      block={mobile}
    >
      {ADD_LABELS[tab]}
    </Button>
  );

  return (
    <div>
      {/* Название раздела живёт в шапке (AppLayout). */}
      {/* На телефоне кнопка добавления уходит из полосы вкладок под неё:
          в полосе она наезжала на сами вкладки, и до них было не
          добраться — оставалось многоточие. */}
      {mobile && addButton && (
        <div style={{ marginBottom: 12 }}>{addButton}</div>
      )}
      <Tabs
        activeKey={tab}
        onChange={setTab}
        // Неактивные вкладки размонтируются: тогда сигнал «добавить»
        // получает ровно одна вкладка — та, что на экране.
        destroyOnHidden
        tabBarExtraContent={!mobile && addButton}
        items={[
          { key: 'executors', label: 'Исполнители', children: <ExecutorsTab addSignal={addSignal} /> },
          { key: 'positions', label: 'Должности', children: <PositionsTab addSignal={addSignal} /> },
          { key: 'work-modes', label: 'Режимы работы', children: <WorkModesTab addSignal={addSignal} /> },
          { key: 'cost-items', label: 'Статьи затрат', children: <CostItemsTab addSignal={addSignal} /> },
        ]}
      />
    </div>
  );
}
