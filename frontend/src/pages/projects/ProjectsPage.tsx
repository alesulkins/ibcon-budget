import React, { useCallback, useEffect, useState } from 'react';
import {
  Table, Button, Input, Select,
  Modal, Form, DatePicker, InputNumber, message,
} from 'antd';
import { PlusOutlined, SearchOutlined } from '@ant-design/icons';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import type { ColumnsType } from 'antd/es/table';
import { useNavigate } from 'react-router-dom';
import dayjs from 'dayjs';
import { projectsApi, refsApi } from '../../api';
import type { ProjectListItem } from '../../types';
import {
  PROJECT_STATUS_LABELS, PROJECT_STATUS_COLORS, BUDGET_STATUS_LABELS, BUDGET_STATUS_COLORS,
} from '../../types';
import { fmtDate, fmtMoney } from '../../utils/fmt';
import Profitability from '../../components/Profitability';
import { capitalizeFirst, normalizeFullName, shortName } from '../../utils/names';
import { PERM, usePermissions } from '../../store/permissions';
import { extractError } from '../../api/client';
import { useStickyState } from '../../hooks/useStickyState';
import { currentUser } from '../../store/auth';
import Fireworks, { shouldShowFireworks, markFireworksShown } from '../../components/Fireworks';
import StatusTag from '../../components/StatusTag';
import { useFillToSiderFooter } from '../../hooks/useFillHeight';

/**
 * Реестр листается не страницами, а прокруткой — вся видимая (по правам)
 * выборка запрашивается одним куском. Проектов в системе на порядки
 * меньше, чем такой лимит: он просто гарантия, что реестр не обрежется
 * молча, если их станет много.
 */
const REGISTRY_LIMIT = 1000;

export default function ProjectsPage() {
  const navigate = useNavigate();
  const qc = useQueryClient();
  const [fillRef, fillHeight] = useFillToSiderFooter<HTMLDivElement>();
  // Фильтры реестра переживают переход в справочники и обратно
  const [search, setSearch] = useStickyState('projects:search', '');
  const [statusFilter, setStatusFilter] = useStickyState<string | undefined>('projects:status', undefined);
  const [budgetStatusFilter, setBudgetStatusFilter] = useStickyState<string | undefined>('projects:budgetStatus', undefined);
  const [executorFilter, setExecutorFilter] = useStickyState<string | undefined>('projects:executor', undefined);
  const [showCreate, setShowCreate] = useState(false);
  const [form] = Form.useForm();
  // Кнопку прячем по праву, а не по роли: главный экономист может
  // выдать создание проектов кому угодно.
  const { can } = usePermissions();
  const canCreate = can(PERM.projectCreate);

  // Салют — раз в день, только для одного пользователя (см. Fireworks)
  const [fireworks, setFireworks] = useState(() => shouldShowFireworks(currentUser()?.full_name));
  useEffect(() => {
    if (fireworks) markFireworksShown();
  }, [fireworks]);
  // Стабильная ссылка обязательна: Fireworks держит onDone в зависимостях
  // эффекта, и новая функция на каждый рендер перезапускала анимацию —
  // холст пересоздавался вместе с циклом requestAnimationFrame.
  const hideFireworks = useCallback(() => setFireworks(false), []);

  const { data, isLoading } = useQuery({
    queryKey: ['projects', search, statusFilter],
    queryFn: () => projectsApi.list({ search, status: statusFilter, limit: REGISTRY_LIMIT }),
  });

  const { data: executors } = useQuery({
    queryKey: ['executors'],
    queryFn: () => refsApi.executors(),
  });

  const createMutation = useMutation({
    mutationFn: (vals: Record<string, unknown>) => projectsApi.create({
      name: vals.name as string,
      customer: vals.customer as string,
      executor_id: vals.executor_id as number,
      location: vals.location as string,
      start_date: (vals.start_date as dayjs.Dayjs).format('DD.MM.YYYY'),
      duration_months: vals.duration_months as number,
      director: vals.director as string,
      manager: vals.manager as string,
      administrator: vals.administrator as string,
      economist: vals.economist as string,
      status: vals.status as string,
    }),
    onSuccess: (proj) => {
      qc.invalidateQueries({ queryKey: ['projects'] });
      message.success('Проект создан');
      setShowCreate(false);
      form.resetFields();
      navigate(`/projects/${proj.id}`);
    },
    onError: (e) => message.error(extractError(e)),
  });

  /**
   * Закрытие формы создания. Если пользователь успел что-то ввести —
   * спрашиваем подтверждение, чтобы случайный клик мимо модала или по
   * «Отмена» не стирал заполненную карточку.
   */
  function closeCreate() {
    const touched = Object.values(form.getFieldsValue()).some(
      v => v !== undefined && v !== null && v !== '',
    );
    if (!touched) {
      setShowCreate(false);
      form.resetFields();
      return;
    }
    Modal.confirm({
      title: 'Отменить создание проекта?',
      content: 'Введённые данные не сохранятся.',
      okText: 'Да, отменить',
      cancelText: 'Продолжить заполнение',
      okButtonProps: { danger: true },
      onOk: () => { setShowCreate(false); form.resetFields(); },
    });
  }

  const hasActiveFilters = !!(search || statusFilter || budgetStatusFilter || executorFilter);

  function resetFilters() {
    setSearch('');
    setStatusFilter(undefined);
    setBudgetStatusFilter(undefined);
    setExecutorFilter(undefined);
  }

  // Статус бюджета и исполнителя API не фильтрует — отбираем на клиенте
  // по уже загруженной странице.
  const rows = (data?.items ?? []).filter(p =>
    (!budgetStatusFilter || p.budget_status === budgetStatusFilter)
    && (!executorFilter || p.executor_name === executorFilter));

  /**
   * Порядок колонок задан владельцем 2026-08-27 и менять его нельзя.
   * «№» — не колонка данных, а номер записи, поэтому стоит перед ними.
   *
   * Ширины не задаём: колонка должна быть ровно такой, чтобы значение
   * помещалось в одну строку (nowrap в index.css), а лишняя ширина
   * уходит в горизонтальную прокрутку — scroll x: 'max-content'.
   */
  const columns: ColumnsType<ProjectListItem> = [
    {
      title: '№',
      dataIndex: 'id',
      className: 'ibcon-num',
      sorter: (a, b) => a.id - b.id,
    },
    {
      title: 'Наименование проекта',
      dataIndex: 'name',
      sorter: (a, b) => a.name.localeCompare(b.name, 'ru'),
      // Наименование набрано основным цветом текста, а не цветом ссылки:
      // в колонке из полусотни строк синий столбец перетягивал внимание
      // на себя. Что это ссылка, видно по подчёркиванию при наведении.
      render: (name, r) => (
        <a className="ibcon-link-plain" onClick={() => navigate(`/projects/${r.id}`)}>
          {name}
        </a>
      ),
    },
    {
      title: 'Заказчик',
      dataIndex: 'customer',
      sorter: (a, b) => a.customer.localeCompare(b.customer, 'ru'),
    },
    {
      title: 'Исполнитель',
      dataIndex: 'executor_name',
    },
    {
      title: 'Статус проекта',
      dataIndex: 'status',
      render: (s) => (
        <StatusTag color={PROJECT_STATUS_COLORS[s]}>
          {PROJECT_STATUS_LABELS[s] ?? s}
        </StatusTag>
      ),
    },
    {
      title: 'Статус бюджета',
      dataIndex: 'budget_status',
      render: (s) => (
        <StatusTag color={s ? BUDGET_STATUS_COLORS[s] : 'grey'}>
          {s ? (BUDGET_STATUS_LABELS[s] ?? s) : 'Отсутствует'}
        </StatusTag>
      ),
    },
    {
      title: 'Стоимость без НДС',
      dataIndex: 'cost_no_vat',
      className: 'ibcon-num',
      render: fmtMoney,
      align: 'right',
      sorter: (a, b) => (a.cost_no_vat ?? 0) - (b.cost_no_vat ?? 0),
    },
    {
      title: 'Рентабельность',
      dataIndex: 'profitability',
      className: 'ibcon-num',
      render: (v: number | null | undefined) => <Profitability value={v} />,
      align: 'right',
      sorter: (a, b) => (a.profitability ?? 0) - (b.profitability ?? 0),
    },
    {
      title: 'Директор',
      dataIndex: 'director',
    },
    {
      title: 'Руководитель',
      dataIndex: 'manager',
    },
    {
      title: 'Администратор',
      dataIndex: 'administrator',
    },
    {
      title: 'Экономист',
      dataIndex: 'economist',
    },
    {
      title: 'Дата создания',
      dataIndex: 'created_at',
      className: 'ibcon-num',
      render: fmtDate,
      sorter: (a, b) => a.created_at.localeCompare(b.created_at),
    },
    {
      title: 'Автор',
      dataIndex: 'created_by_name',
      render: (n: string) => shortName(n),
    },
  ];

  return (
    <div>
      {fireworks && <Fireworks onDone={hideFireworks} />}

      {/* Название раздела живёт в шапке (AppLayout). Фильтры и кнопка
          стоят одной строкой: у всех контролов одна высота, кнопка
          прижата к правому краю.

          Поля сжимаются, кнопки — нет. С жёсткими ширинами на крупном
          шрифте строка переставала помещаться, и кнопка «Создать
          проект» съезжала на вторую строку. */}
      <div style={{
        display: 'flex',
        alignItems: 'center',
        gap: 8,
        flexWrap: 'nowrap',
        marginBottom: 16,
      }}>
        <Input
          prefix={<SearchOutlined />}
          placeholder="Поиск по проекту, заказчику..."
          value={search}
          onChange={e => setSearch(e.target.value)}
          style={{ flex: '2 1 180px', minWidth: 140 }}
          allowClear
        />
        <Select
          placeholder="Статус проекта"
          allowClear
          style={{ flex: '1 1 130px', minWidth: 110 }}
          value={statusFilter}
          onChange={setStatusFilter}
          options={Object.entries(PROJECT_STATUS_LABELS).map(([value, label]) => ({ value, label }))}
        />
        <Select
          placeholder="Статус бюджета"
          allowClear
          style={{ flex: '1 1 130px', minWidth: 110 }}
          value={budgetStatusFilter}
          onChange={setBudgetStatusFilter}
          options={Object.entries(BUDGET_STATUS_LABELS).map(([value, label]) => ({ value, label }))}
        />
        <Select
          placeholder="Исполнитель"
          allowClear
          style={{ flex: '1 1 140px', minWidth: 110 }}
          value={executorFilter}
          onChange={setExecutorFilter}
          options={(executors ?? []).map(e => ({ value: e.name, label: e.name }))}
        />
        <Button
          onClick={resetFilters}
          disabled={!hasActiveFilters}
          style={{ flexShrink: 0 }}
        >
          Сбросить фильтры
        </Button>

        {canCreate && (
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => setShowCreate(true)}
            // marginLeft: auto — кнопка уходит вправо, а фильтры остаются
            // слева. flexShrink: 0 — подпись не должна ужиматься и
            // переноситься, сжимаются поля.
            style={{ marginLeft: 'auto', flexShrink: 0 }}
          >
            Создать проект
          </Button>
        )}
      </div>

      {/* Страниц нет: вся выборка (в пределах прав и фильтров) грузится
          одним запросом, а прокручивается сама таблица — scroll.y
          ограничивает её высотой до линии ЛК в сайдбаре
          (useFillToSiderFooter), страница вниз не растёт. */}
      <div ref={fillRef}>
        <Table
          rowKey="id"
          className="nowrap-table"
          columns={columns}
          dataSource={rows}
          loading={isLoading}
          // max-content, а не фиксированная ширина: таблица становится ровно
          // такой, чтобы ни одно значение не переносилось, независимо от
          // масштаба окна.
          scroll={{ x: 'max-content', y: fillHeight }}
          size="small"
          pagination={false}
        />
      </div>

      <Modal
        title="Создание проекта"
        open={showCreate}
        onCancel={closeCreate}
        onOk={() => form.submit()}
        confirmLoading={createMutation.isPending}
        width={640}
        okText="Создать"
        cancelText="Отмена"
        mask={{ closable: false }}
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={createMutation.mutate}
          initialValues={{ status: 'prospect' }}
        >
          <Form.Item name="name" label="Наименование проекта" rules={[{ required: true, message: 'Не заполнено обязательное поле: Наименование проекта' }]}>
            <Input onBlur={(e) => form.setFieldValue('name', capitalizeFirst(e.target.value))} />
          </Form.Item>
          <Form.Item name="customer" label="Заказчик" rules={[{ required: true, message: 'Не заполнено обязательное поле: Заказчик' }]}>
            <Input onBlur={(e) => form.setFieldValue('customer', capitalizeFirst(e.target.value))} />
          </Form.Item>
          <Form.Item name="executor_id" label="Исполнитель" rules={[{ required: true, message: 'Не заполнено обязательное поле: Исполнитель' }]}>
            <Select
              options={executors?.filter(e => e.active).map(e => ({ value: e.id, label: e.name }))}
            />
          </Form.Item>
          <Form.Item name="location" label="Местонахождение объекта" rules={[{ required: true, message: 'Не заполнено обязательное поле: Местонахождение объекта' }]}>
            <Input onBlur={(e) => form.setFieldValue('location', capitalizeFirst(e.target.value))} />
          </Form.Item>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
            <Form.Item name="start_date" label="Дата начала" rules={[{ required: true, message: 'Не заполнено обязательное поле: Дата начала' }]}>
              <DatePicker format="DD.MM.YYYY" style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="duration_months" label="Продолжительность (мес.)" rules={[{ required: true, message: 'Не заполнено обязательное поле: Продолжительность (мес.)' }]}>
              <InputNumber min={1} max={60} style={{ width: '100%' }} />
            </Form.Item>
          </div>
          <Form.Item
            name="status"
            label="Статус проекта"
            rules={[{ required: true, message: 'Не заполнено обязательное поле: Статус проекта' }]}
          >
            <Select
              options={Object.entries(PROJECT_STATUS_LABELS).map(([value, label]) => ({ value, label }))}
            />
          </Form.Item>

          {/* ФИО приводятся к «Фамилия И.О.» при потере фокуса. Те же
              правила продублированы на сервере — форма лишь показывает
              результат сразу. */}
          {[
            { name: 'director', label: 'Директор проекта' },
            { name: 'manager', label: 'Руководитель проекта' },
            { name: 'administrator', label: 'Администратор проекта' },
            { name: 'economist', label: 'Экономист проекта' },
          ].map(({ name, label }) => (
            <Form.Item
              key={name}
              name={name}
              label={label}
              rules={[{ required: true, message: `Не заполнено обязательное поле: ${label}` }]}
            >
              <Input
                placeholder="Фамилия И.О."
                onBlur={(e) => form.setFieldValue(name, normalizeFullName(e.target.value))}
              />
            </Form.Item>
          ))}
        </Form>
      </Modal>
    </div>
  );
}
