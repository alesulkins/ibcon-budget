import React, { useState } from 'react';
import {
  Card, Descriptions, Tag, Button, Space, Modal, Form, Select,
  Input, Table, Typography, message, Tooltip, DatePicker, InputNumber,
  Popconfirm,
} from 'antd';
import {
  EditOutlined, PlusOutlined, ArrowLeftOutlined,
} from '@ant-design/icons';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import type { ColumnsType } from 'antd/es/table';
import { useParams, useNavigate } from 'react-router-dom';
import dayjs from 'dayjs';
import { projectsApi, budgetsApi, refsApi } from '../../api';
import type { BudgetVersion } from '../../types';
import {
  PROJECT_STATUS_LABELS, PROJECT_STATUS_COLORS, BUDGET_STATUS_LABELS, BUDGET_STATUS_COLORS,
} from '../../types';
import { fmtDate, fmtMoney, fmtPct } from '../../utils/fmt';
import { hasRole } from '../../store/auth';
import { extractError } from '../../api/client';

const { Title, Text } = Typography;

export default function ProjectDetailPage() {
  const { id } = useParams<{ id: string }>();
  const pid = Number(id);
  const navigate = useNavigate();
  const qc = useQueryClient();

  const [showEdit, setShowEdit] = useState(false);
  const [showCreateBudget, setShowCreateBudget] = useState(false);
  const [showStatusModal, setShowStatusModal] = useState(false);
  const [statusForm] = Form.useForm();
  const [editForm] = Form.useForm();
  const [budgetForm] = Form.useForm();

  const { data: project, isLoading } = useQuery({
    queryKey: ['project', pid],
    queryFn: () => projectsApi.get(pid),
  });

  const { data: versions, isLoading: versionsLoading } = useQuery({
    queryKey: ['budget-versions', pid],
    queryFn: () => budgetsApi.listVersions(pid),
  });

  const { data: executors } = useQuery({
    queryKey: ['executors'],
    queryFn: refsApi.executors,
  });

  const updateMutation = useMutation({
    mutationFn: (vals: Record<string, unknown>) => projectsApi.update(pid, {
      name: vals.name as string,
      customer: vals.customer as string,
      executor_id: vals.executor_id as number,
      location: vals.location as string,
      start_date: vals.start_date ? (vals.start_date as dayjs.Dayjs).format('DD.MM.YYYY') : undefined,
      duration_months: vals.duration_months as number,
      director: vals.director as string,
      manager: vals.manager as string,
      administrator: vals.administrator as string,
      economist: vals.economist as string,
    }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['project', pid] });
      qc.invalidateQueries({ queryKey: ['projects'] });
      message.success('Проект обновлён');
      setShowEdit(false);
    },
    onError: (e) => message.error(extractError(e)),
  });

  const statusMutation = useMutation({
    mutationFn: ({ status, comment }: { status: string; comment: string }) =>
      projectsApi.changeStatus(pid, status, comment),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['project', pid] });
      qc.invalidateQueries({ queryKey: ['projects'] });
      message.success('Статус изменён');
      setShowStatusModal(false);
    },
    onError: (e) => message.error(extractError(e)),
  });

  const createBudgetMutation = useMutation({
    mutationFn: (vals: { copy_from_id?: number; comment?: string }) => {
      const hasVersions = (versions ?? []).length > 0;
      if (hasVersions) {
        return budgetsApi.newVersion(pid, vals);
      }
      return budgetsApi.createVersion(pid, vals);
    },
    onSuccess: (v) => {
      qc.invalidateQueries({ queryKey: ['budget-versions', pid] });
      message.success('Версия бюджета создана');
      setShowCreateBudget(false);
      budgetForm.resetFields();
      navigate(`/budget-versions/${v.id}`);
    },
    onError: (e) => message.error(extractError(e)),
  });

  const canEdit = hasRole('GE', 'EP', 'IP');
  const isGE = hasRole('GE');

  const validNextStatuses: Record<string, string[]> = {
    prospect: ['active', 'suspended', 'completed', 'unrealized'],
    active: ['prospect', 'suspended', 'completed', 'unrealized'],
    suspended: ['prospect', 'active', 'completed', 'unrealized'],
    completed: ['prospect', 'active'],
    unrealized: ['prospect', 'active'],
  };

  const versionColumns: ColumnsType<BudgetVersion> = [
    {
      title: 'Версия',
      dataIndex: 'version_label',
      width: 80,
      render: (v) => v ?? '—',
    },
    {
      title: 'Статус',
      dataIndex: 'status',
      width: 140,
      render: (s) => (
        <Tag color={BUDGET_STATUS_COLORS[s]}>{BUDGET_STATUS_LABELS[s] ?? s}</Tag>
      ),
    },
    {
      title: 'Стоимость без НДС',
      dataIndex: 'cost_no_vat',
      render: fmtMoney,
      align: 'right',
    },
    {
      title: 'Рентабельность',
      dataIndex: 'profitability',
      render: fmtPct,
      align: 'right',
    },
    {
      title: 'Дата создания',
      dataIndex: 'created_at',
      render: fmtDate,
    },
    {
      title: 'Автор',
      dataIndex: 'created_by_name',
    },
    {
      title: 'Комментарий',
      dataIndex: 'comment',
      ellipsis: true,
    },
    {
      title: '',
      key: 'actions',
      width: 100,
      render: (_, r) => (
        <Button
          size="small"
          onClick={() => navigate(`/budget-versions/${r.id}`)}
        >
          Открыть
        </Button>
      ),
    },
  ];

  if (isLoading || !project) return null;

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button icon={<ArrowLeftOutlined />} onClick={() => navigate('/projects')}>
          К реестру
        </Button>
      </Space>

      <Card
        title={
          <Space>
            <Title level={4} style={{ margin: 0 }}>{project.name}</Title>
            <Tag color={PROJECT_STATUS_COLORS[project.status]}>
              {PROJECT_STATUS_LABELS[project.status]}
            </Tag>
          </Space>
        }
        extra={
          <Space>
            {isGE && (
              <Button
                onClick={() => setShowStatusModal(true)}
              >
                Изменить статус
              </Button>
            )}
            {canEdit && (
              <Button
                icon={<EditOutlined />}
                onClick={() => {
                  editForm.setFieldsValue({
                    ...project,
                    start_date: dayjs(project.start_date),
                  });
                  setShowEdit(true);
                }}
              >
                Редактировать
              </Button>
            )}
          </Space>
        }
        style={{ marginBottom: 24 }}
      >
        <Descriptions column={2} size="small">
          <Descriptions.Item label="Заказчик">{project.customer}</Descriptions.Item>
          <Descriptions.Item label="Исполнитель">{project.executor_name}</Descriptions.Item>
          <Descriptions.Item label="Местонахождение">{project.location}</Descriptions.Item>
          <Descriptions.Item label="Дата начала">{fmtDate(project.start_date)}</Descriptions.Item>
          <Descriptions.Item label="Продолжительность">{project.duration_months} мес.</Descriptions.Item>
          <Descriptions.Item label="Дата окончания">{fmtDate(project.end_date)}</Descriptions.Item>
          <Descriptions.Item label="Директор">{project.director}</Descriptions.Item>
          <Descriptions.Item label="Руководитель проекта">{project.manager}</Descriptions.Item>
          <Descriptions.Item label="Администратор">{project.administrator}</Descriptions.Item>
          <Descriptions.Item label="Экономист">{project.economist}</Descriptions.Item>
          <Descriptions.Item label="Создан">{fmtDate(project.created_at)}</Descriptions.Item>
          <Descriptions.Item label="Автор">{project.created_by_name}</Descriptions.Item>
        </Descriptions>
      </Card>

      <Card
        title="Версии бюджета"
        extra={
          canEdit && (
            <Button
              type="primary"
              icon={<PlusOutlined />}
              style={{ background: '#1a3a6b' }}
              onClick={() => setShowCreateBudget(true)}
            >
              {(versions ?? []).length > 0 ? 'Новая версия' : 'Создать бюджет'}
            </Button>
          )
        }
      >
        <Table
          rowKey="id"
          columns={versionColumns}
          dataSource={versions ?? []}
          loading={versionsLoading}
          size="small"
          pagination={false}
        />
      </Card>

      {/* Модал изменения статуса */}
      <Modal
        title="Изменение статуса проекта"
        open={showStatusModal}
        onCancel={() => setShowStatusModal(false)}
        onOk={() => statusForm.submit()}
        confirmLoading={statusMutation.isPending}
        okText="Изменить"
        cancelText="Отмена"
      >
        <Form form={statusForm} layout="vertical" onFinish={statusMutation.mutate}>
          <Form.Item name="status" label="Новый статус" rules={[{ required: true }]}>
            <Select
              options={(validNextStatuses[project.status] ?? []).map(s => ({
                value: s,
                label: PROJECT_STATUS_LABELS[s] ?? s,
              }))}
            />
          </Form.Item>
          <Form.Item name="comment" label="Причина изменения" rules={[{ required: true }]}>
            <Input.TextArea rows={3} />
          </Form.Item>
        </Form>
      </Modal>

      {/* Модал создания версии бюджета */}
      <Modal
        title={(versions ?? []).length > 0 ? 'Новая версия бюджета' : 'Создание бюджета'}
        open={showCreateBudget}
        onCancel={() => { setShowCreateBudget(false); budgetForm.resetFields(); }}
        onOk={() => budgetForm.submit()}
        confirmLoading={createBudgetMutation.isPending}
        okText="Создать"
        cancelText="Отмена"
      >
        <Form form={budgetForm} layout="vertical" onFinish={createBudgetMutation.mutate}>
          {(versions ?? []).length > 0 && (
            <Form.Item name="copy_from_id" label="Копировать из версии (необязательно)">
              <Select
                allowClear
                options={(versions ?? [])
                  .filter(v => v.status !== 'archive' || true)
                  .map(v => ({
                    value: v.id,
                    label: `${v.version_label ?? 'Текущая'} (${BUDGET_STATUS_LABELS[v.status]}) — ${fmtDate(v.created_at)}`,
                  }))}
              />
            </Form.Item>
          )}
          <Form.Item name="comment" label="Комментарий">
            <Input.TextArea rows={3} placeholder="Причина создания новой версии..." />
          </Form.Item>
        </Form>
      </Modal>

      {/* Модал редактирования проекта */}
      <Modal
        title="Редактирование проекта"
        open={showEdit}
        onCancel={() => setShowEdit(false)}
        onOk={() => editForm.submit()}
        confirmLoading={updateMutation.isPending}
        width={640}
        okText="Сохранить"
        cancelText="Отмена"
      >
        <Form form={editForm} layout="vertical" onFinish={updateMutation.mutate}>
          <Form.Item name="name" label="Наименование" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="customer" label="Заказчик" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="executor_id" label="Исполнитель" rules={[{ required: true }]}>
            <Select
              options={executors?.filter(e => e.active).map(e => ({ value: e.id, label: e.name }))}
            />
          </Form.Item>
          <Form.Item name="location" label="Местонахождение" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
            <Form.Item name="start_date" label="Дата начала" rules={[{ required: true }]}>
              <DatePicker format="DD.MM.YYYY" style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="duration_months" label="Продолжительность (мес.)" rules={[{ required: true }]}>
              <InputNumber min={1} max={60} style={{ width: '100%' }} />
            </Form.Item>
          </div>
          <Form.Item name="director" label="Директор" rules={[{ required: true }]}>
            <Input placeholder="Фамилия И.О." />
          </Form.Item>
          <Form.Item name="manager" label="Руководитель проекта" rules={[{ required: true }]}>
            <Input placeholder="Фамилия И.О." />
          </Form.Item>
          <Form.Item name="administrator" label="Администратор" rules={[{ required: true }]}>
            <Input placeholder="Фамилия И.О." />
          </Form.Item>
          <Form.Item name="economist" label="Экономист" rules={[{ required: true }]}>
            <Input placeholder="Фамилия И.О." />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
