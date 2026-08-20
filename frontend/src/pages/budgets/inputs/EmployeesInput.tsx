import React, { useEffect, useState } from 'react';
import {
  Card, Table, Button, Modal, Form, Input, Select, InputNumber,
  Space, message, Typography, Tooltip,
} from 'antd';
import { PlusOutlined, DeleteOutlined, SaveOutlined, EditOutlined, ScheduleOutlined } from '@ant-design/icons';
import { useQuery, useMutation } from '@tanstack/react-query';
import type { ColumnsType } from 'antd/es/table';
import dayjs from 'dayjs';
import { budgetsApi } from '../../../api';
import type { Employee, InputEmployees } from '../../../types';
import { extractError } from '../../../api/client';

const { Text } = Typography;

// Условия работы сотрудника (лист 3.Сотрудники, колонка «Условия»)
const BASE_CONDITIONS = ['вахта', 'офис', 'не принят'];

// Коды помесячного графика (лист 4.6)
const MONTHLY_MODES = ['4/2', '4/4', 'ОФ', 'МВ', 'К', 'ОТП', 'не принят'];

interface Props {
  versionId: number;
  duration: number;
  startDate: string;
  readonly?: boolean;
}

const DEFAULT_EMP: InputEmployees = {
  ticket_price: 40_000,
  per_diem_rf: 1148,
  per_diem_other: 2_500,
  employees: [],
};

function monthLabel(startDate: string, idx: number): string {
  return dayjs(startDate).add(idx, 'month').format('MM.YY');
}

export default function EmployeesInput({ versionId, duration, startDate, readonly }: Props) {
  const [data, setData] = useState<InputEmployees>(DEFAULT_EMP);
  const [showAddModal, setShowAddModal] = useState(false);
  const [editingIdx, setEditingIdx] = useState<number | null>(null);
  const [editingEmp, setEditingEmp] = useState<Employee | null>(null);
  const [showScheduleModal, setShowScheduleModal] = useState(false);
  const [schedEmpIdx, setSchedEmpIdx] = useState<number | null>(null);
  const [empForm] = Form.useForm();

  const { data: savedData, isLoading } = useQuery({
    queryKey: ['budget-input', versionId, 'employees'],
    queryFn: () => budgetsApi.getInput<InputEmployees>(versionId, 'employees'),
  });

  useEffect(() => {
    if (savedData && savedData.employees) {
      setData({
        ticket_price: savedData.ticket_price ?? 40_000,
        per_diem_rf: savedData.per_diem_rf ?? 1148,
        per_diem_other: savedData.per_diem_other ?? 2_500,
        employees: savedData.employees,
      });
    }
  }, [savedData, duration]);

  const saveMutation = useMutation({
    mutationFn: () => budgetsApi.saveInput(versionId, 'employees', data),
    onSuccess: () => message.success('Данные по сотрудникам сохранены'),
    onError: (e) => message.error(extractError(e)),
  });

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
      render: (v) => v.toLocaleString('ru-RU'),
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
              <Button size="small" danger icon={<DeleteOutlined />} onClick={() => removeEmployee(idx)} />
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
          <div>
            <Text type="secondary">Стоимость авиабилета, ₽: </Text>
            <InputNumber
              value={data.ticket_price}
              min={0}
              disabled={readonly}
              style={{ width: 140 }}
              onChange={(v) => setData({ ...data, ticket_price: v ?? 40_000 })}
              formatter={(v) => `${v}`.replace(/\B(?=(\d{3})+(?!\d))/g, ' ')}
            />
          </div>
          <div>
            <Text type="secondary">Командировочные по РФ (суточные), ₽: </Text>
            <InputNumber
              value={data.per_diem_rf}
              min={0}
              disabled={readonly}
              style={{ width: 140 }}
              onChange={(v) => setData({ ...data, per_diem_rf: v ?? 1148 })}
              formatter={(v) => `${v}`.replace(/\B(?=(\d{3})+(?!\ด))/g, ' ')}
            />
          </div>
          <div>
            <Text type="secondary">Командировочные за рубеж (суточные), ₽: </Text>
            <InputNumber
              value={data.per_diem_other}
              min={0}
              disabled={readonly}
              style={{ width: 140 }}
              onChange={(v) => setData({ ...data, per_diem_other: v ?? 2_500 })}
              formatter={(v) => `${v}`.replace(/\B(?=(\d{3})+(?!\d))/g, ' ')}
            />
          </div>
        </Space>

        <Text type="secondary" style={{ fontSize: 12 }}>
          Кол-во авиабилетов рассчитывается автоматически из графика: 4/2 → 2 билета, К → 2 билета, смена графика → 1 билет.
        </Text>
      </Card>

      {/* Список сотрудников — лист 3.Сотрудники */}
      <Card
        title={`Сотрудники (${data.employees.length}) — лист 3.Сотрудники`}
        size="small"
        extra={
          <Space>
            {!readonly && (
              <Button
                size="small"
                type="primary"
                icon={<PlusOutlined />}
                onClick={() => { empForm.resetFields(); setEditingIdx(null); setShowAddModal(true); }}
                style={{ background: '#1a3a6b' }}
              >
                Добавить сотрудника
              </Button>
            )}
            {!readonly && (
              <Button
                size="small"
                icon={<SaveOutlined />}
                onClick={() => saveMutation.mutate()}
                loading={saveMutation.isPending}
              >
                Сохранить всё
              </Button>
            )}
          </Space>
        }
      >
        <Table
          rowKey={(_, i) => i!}
          columns={columns}
          dataSource={data.employees}
          loading={isLoading}
          size="small"
          pagination={false}
          locale={{ emptyText: 'Нет сотрудников. Нажмите «Добавить сотрудника».' }}
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
          <Form.Item name="country" label="Страна НО" rules={[{ required: true, message: 'Укажите страну НО' }]}>
            <Select options={[
              { value: 'россия', label: 'Россия' },
              { value: 'киргизия', label: 'Киргизия' },
            ]} />
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
        width={900}
        okText="Сохранить"
        cancelText="Отмена"
      >
        {editingEmp && (
          <div>
            {/* График релокации */}
            <div style={{ marginBottom: 8 }}>
              <Text strong>График релокации / условий работы по месяцам:</Text>
              <div style={{ marginBottom: 6 }}>
                <Text type="secondary" style={{ fontSize: 12 }}>Применить ко всем месяцам: </Text>
                <Select
                  size="small"
                  style={{ width: 120 }}
                  options={MONTHLY_MODES.map(m => ({ value: m, label: m }))}
                  onChange={setApplyToAll}
                  disabled={readonly}
                />
              </div>
              <div style={{ overflowX: 'auto' }}>
                <table style={{ borderCollapse: 'collapse' }}>
                  <thead>
                    <tr>
                      {Array.from({ length: duration }).map((_, i) => (
                        <th key={i} style={{ padding: '2px 4px', fontSize: 11, color: '#888', textAlign: 'center', minWidth: 72 }}>
                          {monthLabel(startDate, i)}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    <tr>
                      {Array.from({ length: duration }).map((_, i) => (
                        <td key={i} style={{ padding: '2px 4px' }}>
                          <Select
                            size="small"
                            style={{ width: 70 }}
                            value={editingEmp.monthly_schedule[i] ?? 'не принят'}
                            disabled={readonly}
                            onChange={(v) => setMonthSchedule(i, v)}
                            options={MONTHLY_MODES.map(m => ({ value: m, label: m }))}
                          />
                        </td>
                      ))}
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>

            {/* Командировочные дни РФ */}
            <div style={{ marginTop: 16 }}>
              <Text strong>Кол-во дней командировок по РФ (дней в месяц):</Text>
              <div style={{ overflowX: 'auto', marginTop: 4 }}>
                <table style={{ borderCollapse: 'collapse' }}>
                  <tbody>
                    <tr>
                      {Array.from({ length: duration }).map((_, i) => (
                        <td key={i} style={{ padding: '2px 4px' }}>
                          <InputNumber
                            size="small"
                            style={{ width: 60 }}
                            value={editingEmp.trip_days_rf[i] ?? 0}
                            min={0}
                            max={31}
                            disabled={readonly}
                            onChange={(v) => setTripDaysRF(i, v ?? 0)}
                          />
                        </td>
                      ))}
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>

            {/* Командировочные дни за рубеж */}
            <div style={{ marginTop: 16 }}>
              <Text strong>Кол-во дней командировок за рубеж (дней в месяц):</Text>
              <div style={{ overflowX: 'auto', marginTop: 4 }}>
                <table style={{ borderCollapse: 'collapse' }}>
                  <tbody>
                    <tr>
                      {Array.from({ length: duration }).map((_, i) => (
                        <td key={i} style={{ padding: '2px 4px' }}>
                          <InputNumber
                            size="small"
                            style={{ width: 60 }}
                            value={editingEmp.trip_days_other[i] ?? 0}
                            min={0}
                            max={31}
                            disabled={readonly}
                            onChange={(v) => setTripDaysOther(i, v ?? 0)}
                          />
                        </td>
                      ))}
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
}
