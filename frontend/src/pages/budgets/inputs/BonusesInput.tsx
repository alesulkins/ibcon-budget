import React, { useEffect, useState } from 'react';
import {
  Card, Table, Button, Modal, Form, Input, InputNumber,
  Select, message, Space, Typography, Tag,
} from 'antd';
import { PlusOutlined, DeleteOutlined, SaveOutlined } from '@ant-design/icons';
import { useQuery, useMutation } from '@tanstack/react-query';
import type { ColumnsType } from 'antd/es/table';
import dayjs from 'dayjs';
import { budgetsApi } from '../../../api';
import type { BonusType, InputBonuses, InputEmployees } from '../../../types';
import { extractError } from '../../../api/client';

const { Text } = Typography;

const MONTH_NAMES = ['Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь',
  'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь'];

interface Props {
  versionId: number;
  duration: number;
  startDate: string;
  readonly?: boolean;
}

function monthLabel(startDate: string, idx: number): string {
  return dayjs(startDate).add(idx, 'month').format('MM.YY');
}

// Находит индекс месяца проекта (0-based) по номеру месяца в году
function findProjectMonthIdx(startDate: string, monthNum: number, duration: number): number | null {
  for (let i = 0; i < duration; i++) {
    const m = dayjs(startDate).add(i, 'month').month() + 1; // 1-12
    if (m === monthNum) return i;
  }
  return null;
}

// Вычисляет суммы премий по сотруднику на основе типов премий
function computeBonusAmounts(
  salary: number,
  bonusTypes: BonusType[],
  startDate: string,
  duration: number,
): number[] {
  const amounts = Array(duration).fill(0);
  for (const bt of bonusTypes) {
    const idx = findProjectMonthIdx(startDate, bt.month_num, duration);
    if (idx !== null) {
      amounts[idx] += salary * bt.pct_of_salary;
    }
  }
  return amounts;
}

export default function BonusesInput({ versionId, duration, startDate, readonly }: Props) {
  const [bonusTypes, setBonusTypes] = useState<BonusType[]>([]);
  const [showAdd, setShowAdd] = useState(false);
  const [editingIdx, setEditingIdx] = useState<number | null>(null);
  const [form] = Form.useForm();

  const { data: savedBonuses } = useQuery({
    queryKey: ['budget-input', versionId, 'bonuses'],
    queryFn: () => budgetsApi.getInput<InputBonuses>(versionId, 'bonuses'),
  });

  const { data: savedEmployees } = useQuery({
    queryKey: ['budget-input', versionId, 'employees'],
    queryFn: () => budgetsApi.getInput<InputEmployees>(versionId, 'employees'),
  });

  useEffect(() => {
    if (savedBonuses?.bonus_types) {
      setBonusTypes(savedBonuses.bonus_types);
    }
  }, [savedBonuses]);

  const employees = savedEmployees?.employees ?? [];

  const saveMutation = useMutation({
    mutationFn: () => {
      // Вычисляем суммы по каждому сотруднику и сохраняем
      const computedEmployees = employees.map(emp => ({
        full_name: emp.full_name || emp.position,
        country: emp.country,
        monthly_amounts: computeBonusAmounts(emp.salary_net, bonusTypes, startDate, duration),
      }));
      const payload: InputBonuses = {
        bonus_types: bonusTypes,
        employees: computedEmployees,
      };
      return budgetsApi.saveInput(versionId, 'bonuses', payload);
    },
    onSuccess: () => message.success('Данные по премиям сохранены'),
    onError: (e) => message.error(extractError(e)),
  });

  function addOrEditBonus(vals: { name: string; month_num: number; pct_of_salary: number }) {
    const bt: BonusType = {
      name: vals.name,
      month_num: vals.month_num,
      pct_of_salary: vals.pct_of_salary / 100, // пользователь вводит %, храним долю
    };
    if (editingIdx !== null) {
      const next = [...bonusTypes];
      next[editingIdx] = bt;
      setBonusTypes(next);
    } else {
      setBonusTypes([...bonusTypes, bt]);
    }
    setShowAdd(false);
    form.resetFields();
    setEditingIdx(null);
  }

  function removeBonus(idx: number) {
    setBonusTypes(bonusTypes.filter((_, i) => i !== idx));
  }

  const bonusTypeColumns: ColumnsType<BonusType> = [
    { title: '№', render: (_, __, i) => i + 1, width: 40 },
    { title: 'Наименование премии', dataIndex: 'name' },
    {
      title: 'Мес. (№ в году)',
      dataIndex: 'month_num',
      width: 130,
      render: (v: number) => `${v} — ${MONTH_NAMES[v - 1] ?? ''}`,
    },
    {
      title: '% от ЗП',
      dataIndex: 'pct_of_salary',
      width: 100,
      align: 'right',
      render: (v: number) => `${(v * 100).toFixed(0)} %`,
    },
    {
      title: '',
      key: 'actions',
      width: 80,
      render: (_, bt, idx) => !readonly && (
        <Space size={4}>
          <Button
            size="small"
            onClick={() => {
              form.setFieldsValue({ ...bt, pct_of_salary: bt.pct_of_salary * 100 });
              setEditingIdx(idx);
              setShowAdd(true);
            }}
          >
            ✏
          </Button>
          <Button size="small" danger icon={<DeleteOutlined />} onClick={() => removeBonus(idx)} />
        </Space>
      ),
    },
  ];

  // Месяцы проекта для заголовков
  const projectMonths = Array.from({ length: duration }, (_, i) => monthLabel(startDate, i));

  return (
    <div>
      {/* Таблица типов премий — верхняя часть листа 4.1 */}
      <Card
        title="Виды премий — лист 4.1"
        size="small"
        style={{ marginBottom: 16 }}
        extra={
          !readonly && (
            <Space>
              <Button
                size="small"
                type="primary"
                icon={<PlusOutlined />}
                onClick={() => { form.resetFields(); setEditingIdx(null); setShowAdd(true); }}
                style={{ background: '#1a3a6b' }}
              >
                Добавить вид премии
              </Button>
              <Button
                size="small"
                icon={<SaveOutlined />}
                onClick={() => saveMutation.mutate()}
                loading={saveMutation.isPending}
              >
                Сохранить
              </Button>
            </Space>
          )
        }
      >
        <Text type="secondary" style={{ display: 'block', marginBottom: 12, fontSize: 12 }}>
          Укажите все месяцы, в которые предполагается премирование, и % от ЗП сотрудника (лист 4.1 Excel).
          Суммы по сотрудникам рассчитываются автоматически.
        </Text>
        <Table
          rowKey={(_, i) => i!}
          columns={bonusTypeColumns}
          dataSource={bonusTypes}
          size="small"
          pagination={false}
          locale={{ emptyText: 'Нет видов премий. Нажмите «Добавить вид премии».' }}
        />
      </Card>

      {/* Расчётная таблица по сотрудникам */}
      {employees.length > 0 && bonusTypes.length > 0 && (
        <Card title="Расчётные суммы премий по сотрудникам" size="small">
          <Text type="secondary" style={{ display: 'block', marginBottom: 8, fontSize: 12 }}>
            Суммы рассчитаны автоматически: Оклад × % от ЗП в указанный месяц.
          </Text>
          <div style={{ overflowX: 'auto' }}>
            <table style={{ borderCollapse: 'collapse', fontSize: 12, width: '100%' }}>
              <thead>
                <tr>
                  <th style={{ padding: '4px 8px', textAlign: 'left', minWidth: 160, borderBottom: '1px solid #f0f0f0' }}>
                    Сотрудник
                  </th>
                  <th style={{ padding: '4px 8px', textAlign: 'left', width: 80, borderBottom: '1px solid #f0f0f0' }}>
                    Страна НО
                  </th>
                  {projectMonths.map((m, i) => (
                    <th key={i} style={{ padding: '4px 6px', textAlign: 'right', minWidth: 80, color: '#888', fontWeight: 400, borderBottom: '1px solid #f0f0f0' }}>
                      {m}
                    </th>
                  ))}
                  <th style={{ padding: '4px 8px', textAlign: 'right', minWidth: 100, borderBottom: '1px solid #f0f0f0' }}>
                    Итого
                  </th>
                </tr>
              </thead>
              <tbody>
                {employees.map((emp, i) => {
                  const amounts = computeBonusAmounts(emp.salary_net, bonusTypes, startDate, duration);
                  const total = amounts.reduce((s, v) => s + v, 0);
                  return (
                    <tr key={i} style={{ borderTop: '1px solid #f9f9f9' }}>
                      <td style={{ padding: '3px 8px' }}>
                        <div style={{ fontWeight: 500 }}>{emp.position}</div>
                        {emp.full_name && <div style={{ color: '#888', fontSize: 11 }}>{emp.full_name}</div>}
                      </td>
                      <td style={{ padding: '3px 8px' }}>
                        <Tag color={emp.country === 'россия' ? 'blue' : 'green'} style={{ fontSize: 11 }}>
                          {emp.country}
                        </Tag>
                      </td>
                      {amounts.map((v, mi) => (
                        <td key={mi} style={{ padding: '3px 6px', textAlign: 'right', color: v > 0 ? '#1a3a6b' : '#ccc' }}>
                          {v > 0 ? v.toLocaleString('ru-RU', { maximumFractionDigits: 0 }) : '—'}
                        </td>
                      ))}
                      <td style={{ padding: '3px 8px', textAlign: 'right', fontWeight: 600 }}>
                        {total > 0 ? total.toLocaleString('ru-RU', { maximumFractionDigits: 0 }) : '—'}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </Card>
      )}

      {employees.length === 0 && (
        <Card size="small">
          <Text type="secondary">Сначала добавьте сотрудников на шаге «Сотрудники». Суммы премий рассчитываются автоматически.</Text>
        </Card>
      )}

      {/* Модал добавления/редактирования вида премии */}
      <Modal
        title={editingIdx !== null ? 'Редактировать вид премии' : 'Добавить вид премии'}
        open={showAdd}
        onCancel={() => { setShowAdd(false); form.resetFields(); setEditingIdx(null); }}
        onOk={() => form.submit()}
        okText="Сохранить"
        cancelText="Отмена"
        width={420}
      >
        <Form form={form} layout="vertical" onFinish={addOrEditBonus}>
          <Form.Item name="name" label="Наименование премии" rules={[{ required: true, message: 'Укажите название' }]}>
            <Input placeholder="День строителя (авг), НГ (дек)…" />
          </Form.Item>
          <Form.Item name="month_num" label="Месяц (№ в году)" rules={[{ required: true, message: 'Укажите месяц' }]}>
            <Select
              options={MONTH_NAMES.map((name, i) => ({ value: i + 1, label: `${i + 1} — ${name}` }))}
              placeholder="Выберите месяц"
            />
          </Form.Item>
          <Form.Item
            name="pct_of_salary"
            label="% от ЗП сотрудника"
            rules={[{ required: true, message: 'Укажите %' }]}
            help="Например: 50 = половина оклада, 100 = полный оклад"
          >
            <InputNumber
              style={{ width: '100%' }}
              min={0}
              max={500}
              addonAfter="%"
              placeholder="50"
            />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
