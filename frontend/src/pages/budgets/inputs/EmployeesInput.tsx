import { useCallback, useEffect, useState } from 'react';
import {
  Card, Table, Button, Modal, Form, Input, Select, InputNumber,
  Space, Typography, Tooltip,
} from 'antd';
import { PlusOutlined, EditOutlined, ScheduleOutlined } from '@ant-design/icons';
import { useQuery } from '@tanstack/react-query';
import type { ColumnsType } from 'antd/es/table';
import { budgetsApi } from '../../../api';
import type { Employee, InputEmployees } from '../../../types';
import { monthLabel, thousandFormatter, thousandParser, fmtNum } from '../../../utils/fmt';
import MonthGrid, {
  monthGridCell, monthGridHeadCell, LABEL_COL_WIDTH,
} from '../../../components/MonthGrid';
import DeleteRowButton from '../../../components/DeleteRowButton';
import EmptyBlock from '../../../components/EmptyBlock';
import { useAutosave } from '../../../hooks/useAutosave';

const { Text } = Typography;

// Страны НО — справочник 5.1!H4:H6. Значения совпадают с константами
// бэкенда (calc.CountryRF / CountryKG / CountrySelfEmployed).
const COUNTRY_KG = 'киргизия';
const COUNTRIES = [
  { value: 'россия', label: 'Россия' },
  { value: COUNTRY_KG, label: 'Киргизия' },
  { value: 'самозанятый, без но', label: 'Самозанятый, без НО' },
];

// Условия работы сотрудника (лист 3.Сотрудники, колонка «Условия»)
const BASE_CONDITIONS = ['вахта', 'офис', 'не принят'];

// Код графика «командировка» — от него зависит показ строк с днями поездок
const MODE_TRIP = 'К';

// Коды помесячного графика (лист 4.6)
const MONTHLY_MODES = ['4/2', '4/4', 'ОФ', 'МВ', 'К', 'ОТП', 'не принят'];

// Расшифровка кодов графика — справка рядом с таблицей
const MODE_LEGEND: { code: string; text: string }[] = [
  { code: 'МВ', text: 'междувахтовый отдых' },
  { code: 'К', text: 'командировка' },
  { code: '4/2', text: 'график нахождения на объекте 4 недели через 2' },
  { code: 'ОФ', text: 'вахтовый персонал находится на вахте' },
  { code: '4/4', text: 'график нахождения на объекте 4 недели через 4' },
  { code: 'ОТП', text: 'отпуск' },
  { code: 'не принят', text: 'сотрудник ещё не принят или уже уволен в этом месяце' },
];

interface Props {
  versionId: number;
  duration: number;
  startDate: string;
  /** Исполнитель проекта — от него зависит список доступных стран НО. */
  executor?: string;
  readonly?: boolean;
}

/**
 * Параметры расчёта ФОТ. fallback — значение, которое подставит бэкенд,
 * если поле оставлено пустым (в JSON уходит 0).
 */
const PARAM_FIELDS = [
  { key: 'ticket_price' as const,   label: 'Стоимость авиабилета, ₽',              fallback: 40_000 },
  { key: 'per_diem_other' as const, label: 'Командировочные за рубеж (суточные), ₽', fallback: 2_500 },
];

/**
 * Суточные по РФ — не ввод, а формула формы 4.6!D9 = 700+300/0.87*1.3.
 * Показываем значение только для чтения, чтобы экономист видел, что
 * именно уходит в расчёт.
 */
const PER_DIEM_RF = 700 + 300 / 0.87 * 1.3;

const DEFAULT_EMP: InputEmployees = {
  ticket_price: 40_000,
  per_diem_rf: 1148,
  per_diem_other: 2_500,
  employees: [],
};

export default function EmployeesInput({
  versionId, duration, startDate, executor, readonly,
}: Props) {
  /**
   * Страна НО «Киргизия» допустима только у киргизского исполнителя:
   * взносы Киргизии (2.Бюджет!174) считаются лишь в этой ветке, иначе
   * сотрудник молча остался бы без страховых взносов. Бэкенд проверяет
   * это независимо (calc.ValidateEmployees).
   */
  const isKGExecutor = (executor ?? '').trim().toLowerCase() === 'айбикон киргизия';
  const countryOptions = COUNTRIES.filter(c => isKGExecutor || c.value !== COUNTRY_KG);

  const [data, setData] = useState<InputEmployees>(DEFAULT_EMP);
  const [showAddModal, setShowAddModal] = useState(false);
  const [editingIdx, setEditingIdx] = useState<number | null>(null);
  const [editingEmp, setEditingEmp] = useState<Employee | null>(null);
  const [showScheduleModal, setShowScheduleModal] = useState(false);
  const [schedEmpIdx, setSchedEmpIdx] = useState<number | null>(null);
  const [empForm] = Form.useForm();

  const { data: savedData, isLoading, isSuccess } = useQuery({
    queryKey: ['budget-input', versionId, 'employees'],
    queryFn: () => budgetsApi.getInput<InputEmployees>(versionId, 'employees'),
  });

  const [hydrated, setHydrated] = useState(false);

  useEffect(() => {
    if (!isSuccess) return;
    if (savedData && savedData.employees) {
      setData({
        ticket_price: savedData.ticket_price ?? 40_000,
        per_diem_rf: savedData.per_diem_rf ?? 1148,
        per_diem_other: savedData.per_diem_other ?? 2_500,
        employees: savedData.employees,
      });
    }
    setHydrated(true);
  }, [savedData, duration, isSuccess]);

  const save = useCallback(
    (d: InputEmployees) => budgetsApi.saveInput(versionId, 'employees', d),
    [versionId],
  );

  useAutosave({ data, ready: hydrated, save, enabled: !readonly });

  function addEmployee(vals: Record<string, unknown>) {
    const cond = vals.base_schedule as string;
    const defaultMonthly = cond === 'вахта' ? '4/2' : cond === 'офис' ? 'ОФ' : 'не принят';
    const emp: Employee = {
      position: vals.position as string,
      full_name: vals.full_name as string || '',
      country: vals.country as string,
      base_schedule: cond,
      salary_net: vals.salary_net as number,
      monthly_schedule: Array(duration).fill(defaultMonthly),
      trip_days_rf: Array(duration).fill(0),
      trip_days_other: Array(duration).fill(0),
    };
    if (editingIdx !== null) {
      const emps = [...data.employees];
      emps[editingIdx] = { ...emps[editingIdx], ...emp };
      setData({ ...data, employees: emps });
    } else {
      setData({ ...data, employees: [...data.employees, emp] });
    }
    setShowAddModal(false);
    empForm.resetFields();
    setEditingIdx(null);
  }

  function removeEmployee(idx: number) {
    setData({ ...data, employees: data.employees.filter((_, i) => i !== idx) });
  }

  function openSchedule(idx: number) {
    setSchedEmpIdx(idx);
    setEditingEmp({ ...data.employees[idx] });
    setShowScheduleModal(true);
  }

  function saveSchedule() {
    if (schedEmpIdx === null || !editingEmp) return;
    const emps = [...data.employees];
    emps[schedEmpIdx] = editingEmp;
    setData({ ...data, employees: emps });
    setShowScheduleModal(false);
    setEditingEmp(null);
    setSchedEmpIdx(null);
  }

  function setMonthSchedule(monthIdx: number, value: string) {
    if (!editingEmp) return;
    const sched = [...editingEmp.monthly_schedule];
    while (sched.length < duration) sched.push('не принят');
    sched[monthIdx] = value;
    setEditingEmp({ ...editingEmp, monthly_schedule: sched });
  }

  function setApplyToAll(value: string) {
    if (!editingEmp) return;
    setEditingEmp({ ...editingEmp, monthly_schedule: Array(duration).fill(value) });
  }

  function setTripDaysRF(monthIdx: number, val: number) {
    if (!editingEmp) return;
    const arr = [...editingEmp.trip_days_rf];
    while (arr.length < duration) arr.push(0);
    arr[monthIdx] = val;
    setEditingEmp({ ...editingEmp, trip_days_rf: arr });
  }

  function setTripDaysOther(monthIdx: number, val: number) {
    if (!editingEmp) return;
    const arr = [...editingEmp.trip_days_other];
    while (arr.length < duration) arr.push(0);
    arr[monthIdx] = val;
    setEditingEmp({ ...editingEmp, trip_days_other: arr });
  }

  const columns: ColumnsType<Employee> = [
    { title: '№', render: (_, __, i) => i + 1, width: 40 },
    { title: 'Должность', dataIndex: 'position', width: 160 },
    { title: 'ФИО', dataIndex: 'full_name', render: (v) => v || '—' },
    { title: 'Страна НО', dataIndex: 'country', width: 100 },
    { title: 'Условия', dataIndex: 'base_schedule', width: 90 },
    {
      title: 'План ФОТ на руки, ₽',
      dataIndex: 'salary_net',
      render: (v) => fmtNum(v),
      align: 'right',
      width: 150,
    },
    {
      title: '',
      key: 'actions',
      width: 140,
      render: (_, emp, idx) => (
        <Space size={4}>
          <Tooltip title="График работы и командировки">
            <Button size="small" icon={<ScheduleOutlined />} onClick={() => openSchedule(idx)}>
              График
            </Button>
          </Tooltip>
          {!readonly && (
            <>
              <Button
                size="small"
                icon={<EditOutlined />}
                onClick={() => {
                  empForm.setFieldsValue(emp);
                  setEditingIdx(idx);
                  setShowAddModal(true);
                }}
              />
              <DeleteRowButton
                variant="default"
                title="Удалить сотрудника?"
                onConfirm={() => removeEmployee(idx)}
              />
            </>
          )}
        </Space>
      ),
    },
  ];

  return (
    <div>
      {/* Параметры расчёта ФОТ — лист 4.6 */}
      <Card title="Параметры (авиабилеты и командировочные) — лист 4.6" size="small" style={{ marginBottom: 16 }}>
        <Space wrap size="large" style={{ marginBottom: 16 }}>
          {PARAM_FIELDS.map(({ key, label, fallback }) => (
            <div key={key}>
              <Text type="secondary">{label}: </Text>
              <InputNumber
                // Пустое поле показываем пустым, а не нулём: иначе после
                // очистки в поле остаётся «0», поверх которого неудобно
                // печатать.
                value={data[key] || null}
                min={0}
                disabled={readonly}
                style={{ width: 150 }}
                placeholder={`по умолчанию ${fmtNum(fallback)}`}
                // Раньше здесь стояло `v ?? fallback`, из-за чего стирание
                // последнего символа мгновенно возвращало значение по
                // умолчанию и поле нельзя было очистить. 0 означает
                // «использовать значение по умолчанию» — так это и
                // трактует бэкенд.
                onChange={(v) => setData({ ...data, [key]: v ?? 0 })}
                formatter={thousandFormatter}
                parser={thousandParser}
              />
            </div>
          ))}

          <div>
            <Text type="secondary">Командировочные по РФ (суточные), ₽: </Text>
            <Tooltip title="Формула формы 4.6!D9 = 700 + 300/0.87 × 1.3. Не редактируется; будет вынесено в настройки платформы.">
              <InputNumber
                value={Number(PER_DIEM_RF.toFixed(2))}
                disabled
                style={{ width: 150 }}
              />
            </Tooltip>
          </div>
        </Space>
      </Card>

      {/* Список сотрудников — лист 3.Сотрудники */}
      <Card
        title={`Сотрудники (${data.employees.length}) — лист 3.Сотрудники`}
        size="small"
        extra={
          !readonly && (
            <Button
              size="small"
              type="primary"
              icon={<PlusOutlined />}
              onClick={() => { empForm.resetFields(); setEditingIdx(null); setShowAddModal(true); }}
              style={{ background: '#1a3a6b' }}
            >
              Добавить сотрудника
            </Button>
          )
        }
      >
        <Table
          rowKey={(_, i) => i!}
          columns={columns}
          dataSource={data.employees}
          loading={isLoading}
          size="small"
          pagination={false}
          // Пока строк нет, шапка таблицы не нужна — только подсказка.
          showHeader={data.employees.length > 0}
          locale={{ emptyText: <EmptyBlock /> }}
        />
      </Card>

      {/* Модал добавления / редактирования */}
      <Modal
        title={editingIdx !== null ? 'Редактирование сотрудника' : 'Добавление сотрудника'}
        open={showAddModal}
        onCancel={() => { setShowAddModal(false); empForm.resetFields(); setEditingIdx(null); }}
        onOk={() => empForm.submit()}
        okText="Сохранить"
        cancelText="Отмена"
        width={480}
      >
        <Form form={empForm} layout="vertical" onFinish={addEmployee}>
          <Form.Item name="position" label="Должность (Специалист)" rules={[{ required: true, message: 'Укажите должность' }]}>
            <Input placeholder="Инженер ПТО, Руководитель проекта…" />
          </Form.Item>
          <Form.Item name="full_name" label="ФИО">
            <Input placeholder="Иванов И.И." />
          </Form.Item>
          <Form.Item
            name="country"
            label="Страна НО"
            rules={[{ required: true, message: 'Укажите страну НО' }]}
            extra={isKGExecutor
              ? undefined
              : 'Вариант «Киргизия» доступен только у исполнителя «Айбикон Киргизия».'}
          >
            <Select options={countryOptions} />
          </Form.Item>
          <Form.Item name="base_schedule" label="Условия работы" rules={[{ required: true, message: 'Укажите условия работы' }]}>
            <Select
              options={BASE_CONDITIONS.map(m => ({ value: m, label: m }))}
              placeholder="вахта / офис"
            />
          </Form.Item>
          <Form.Item name="salary_net" label='План ФОТ на руки, ₽' rules={[{ required: true, message: 'Укажите оклад' }]}>
            <InputNumber
              style={{ width: '100%' }}
              min={0}
              formatter={(v) => `${v}`.replace(/\B(?=(\d{3})+(?!\d))/g, ' ')}
              placeholder="200 000"
            />
          </Form.Item>
        </Form>
      </Modal>

      {/* Модал графика работы + командировки по месяцам */}
      <Modal
        title={`График работы: ${editingEmp?.position ?? ''} ${editingEmp?.full_name ?? ''}`}
        open={showScheduleModal}
        onCancel={() => setShowScheduleModal(false)}
        onOk={saveSchedule}
        width={1100}
        okText="Сохранить"
        cancelText="Отмена"
      >
        {editingEmp && (() => {
          // Дни командировок имеют смысл только там, где в графике стоит «К».
          const tripMonths = Array.from({ length: duration })
            .map((_, i) => editingEmp.monthly_schedule[i] === MODE_TRIP);
          const hasTrips = tripMonths.some(Boolean);
          const months = Array.from({ length: duration }, (_, i) => monthLabel(startDate, i));

          // Строка дней командировок рисует ВСЕ месяцы, а поле ввода ставит
          // только в месяцы с «К». Пропускать колонки нельзя — иначе ячейки
          // съезжают и январь оказывается под мартом.
          const tripRow = (
            label: string,
            values: number[],
            onChange: (monthIdx: number, v: number) => void,
          ) => (
            <tr>
              <td style={{ ...monthGridCell, textAlign: 'left', fontSize: 12, whiteSpace: 'nowrap' }}>
                {label}
              </td>
              {months.map((_, i) => (
                <td key={i} style={monthGridCell}>
                  {tripMonths[i] ? (
                    <InputNumber
                      size="small"
                      style={{ width: '100%' }}
                      value={values[i] ?? 0}
                      min={0}
                      max={31}
                      disabled={readonly}
                      onChange={(v) => onChange(i, v ?? 0)}
                    />
                  ) : (
                    <span style={{ color: '#d9d9d9' }}>—</span>
                  )}
                </td>
              ))}
            </tr>
          );

          return (
            <div style={{ display: 'flex', gap: 16, alignItems: 'flex-start' }}>
              {/* Левая колонка: график и командировки */}
              <div style={{ flex: '1 1 auto', minWidth: 0 }}>
                <Text strong>График релокации / условий работы по месяцам:</Text>
                <div style={{ margin: '6px 0' }}>
                  <Text type="secondary" style={{ fontSize: 12 }}>Применить ко всем месяцам: </Text>
                  <Select
                    size="small"
                    style={{ width: 120 }}
                    options={MONTHLY_MODES.map(m => ({ value: m, label: m }))}
                    onChange={setApplyToAll}
                    disabled={readonly}
                  />
                </div>

                <MonthGrid
                  months={months}
                  labelWidth={LABEL_COL_WIDTH}
                  head={(
                    <thead>
                      <tr>
                        <th style={monthGridHeadCell} />
                        {months.map((m, i) => (
                          <th key={i} style={monthGridHeadCell}>{m}</th>
                        ))}
                      </tr>
                    </thead>
                  )}
                >
                  <tbody>
                    <tr>
                      <td style={{ ...monthGridCell, textAlign: 'left', fontSize: 12, whiteSpace: 'nowrap' }}>
                        График
                      </td>
                      {months.map((_, i) => (
                        <td key={i} style={monthGridCell}>
                          <Select
                            size="small"
                            style={{ width: '100%' }}
                            value={editingEmp.monthly_schedule[i] ?? 'не принят'}
                            disabled={readonly}
                            onChange={(v) => setMonthSchedule(i, v)}
                            options={MONTHLY_MODES.map(m => ({ value: m, label: m }))}
                          />
                        </td>
                      ))}
                    </tr>

                    {hasTrips && (
                      <>
                        <tr>
                          <td colSpan={duration + 1} style={{ paddingTop: 14 }}>
                            <Text strong style={{ fontSize: 13 }}>
                              Дни командировок (месяцы с графиком «К»):
                            </Text>
                          </td>
                        </tr>
                        {tripRow('по РФ', editingEmp.trip_days_rf, setTripDaysRF)}
                        {tripRow('за рубеж', editingEmp.trip_days_other, setTripDaysOther)}
                      </>
                    )}
                  </tbody>
                </MonthGrid>

                {!hasTrips && (
                  <Text type="secondary" style={{ fontSize: 12, display: 'block', marginTop: 16 }}>
                    Дни командировок появятся, если поставить в графике «К».
                  </Text>
                )}
              </div>

              {/* Правая колонка: расшифровка кодов графика */}
              <div
                style={{
                  flex: '0 0 240px',
                  background: '#fafafa',
                  border: '1px solid #f0f0f0',
                  borderRadius: 6,
                  padding: '10px 12px',
                }}
              >
                <Text strong style={{ fontSize: 12, display: 'block', marginBottom: 8 }}>
                  Обозначения
                </Text>
                {MODE_LEGEND.map(({ code, text }) => (
                  <div key={code} style={{ display: 'flex', gap: 8, marginBottom: 6, fontSize: 12 }}>
                    <span style={{
                      flex: '0 0 62px',
                      fontWeight: 600,
                      color: '#595959',
                      whiteSpace: 'nowrap',
                    }}>
                      {code}
                    </span>
                    <span style={{ color: '#8c8c8c', lineHeight: 1.4 }}>{text}</span>
                  </div>
                ))}
              </div>
            </div>
          );
        })()}
      </Modal>
    </div>
  );
}
