import React, { useState } from 'react';
import {
  Table, Button, Tag, Space, Input, Select, Typography,
  Modal, Form, DatePicker, InputNumber, message, Tooltip,
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
import { fmtDate, fmtMoney, fmtPct } from '../../utils/fmt';
import { hasRole } from '../../store/auth';
import { extractError } from '../../api/client';

const { Title } = Typography;

export default function ProjectsPage() {
  const navigate = useNavigate();
  const qc = useQueryClient();
  const [search, setSearch] = useState('');
  const [statusFilter, setStatusFilter] = useState<string | undefined>();
  const [page, setPage] = useState(1);
  const [showCreate, setShowCreate] = useState(false);
  const [form] = Form.useForm();
  const canCreate = hasRole('GE', 'IP');

  const { data, isLoading } = useQuery({
    queryKey: ['projects', search, statusFilter, page],
    queryFn: () => projectsApi.list({ search, status: statusFilter, page, limit: 50 }),
  });

  const { data: executors } = useQuery({
    queryKey: ['executors'],
    queryFn: refsApi.executors,
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
      status: 'prospect',
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

  const columns: ColumnsType<ProjectListItem> = [
    {
      title: '№',
      dataIndex: 'id',
      width: 60,
      sorter: (a, b) => a.id - b.id,
    },
    {
      title: 'Наименование проекта',
      dataIndex: 'name',
      render: (name, r) => (
        <a onClick={() => navigate(`/projects/${r.id}`)}>{name}</a>
      ),
    },
    {
      title: 'Заказчик',
      dataIndex: 'customer',
    },
    {
      title: 'Исполнитель',
      dataIndex: 'executor_name',
    },
    {
      title: 'Директор',
      dataIndex: 'director',
    },
    {
      title: 'РП',
      dataIndex: 'manager',
    },
    {
      title: 'Экономист',
      dataIndex: 'economist',
    },
    {
      title: 'Статус проекта',
      dataIndex: 'status',
      render: (s) => (
        <Tag color={PROJECT_STATUS_COLORS[s]}>{PROJECT_STATUS_LABELS[s] ?? s}</Tag>
      ),
    },
    {
      title: 'Статус бюджета',
      dataIndex: 'budget_status',
      render: (s) => s ? (
        <Tag color={BUDGET_STATUS_COLORS[s]}>{BUDGET_STATUS_LABELS[s] ?? s}</Tag>
      ) : <Tag>Отсутствует</Tag>,
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
  ];

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <Title level={4} style={{ margin: 0 }}>Реестр проектов</Title>
        {canCreate && (
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => setShowCreate(true)}
            style={{ background: '#1a3a6b' }}
          >
            Создать проект
          </Button>
        )}
      </div>

      <Space style={{ marginBottom: 16 }}>
        <Input
          prefix={<SearchOutlined />}
          placeholder="Поиск по проекту, заказчику..."
          value={search}
          onChange={e => { setSearch(e.target.value); setPage(1); }}
          style={{ width: 280 }}
          allowClear
        />
        <Select
          placeholder="Статус проекта"
          allowClear
          style={{ width: 180 }}
          value={statusFilter}
          onChange={v => { setStatusFilter(v); setPage(1); }}
          options={Object.entries(PROJECT_STATUS_LABELS).map(([value, label]) => ({ value, label }))}
        />
      </Space>

      <Table
        rowKey="id"
        columns={columns}
        dataSource={data?.items ?? []}
        loading={isLoading}
        scroll={{ x: 1400 }}
        size="small"
        pagination={{
          current: page,
          pageSize: 50,
          total: data?.total ?? 0,
          onChange: setPage,
          showTotal: (total) => `Всего: ${total}`,
        }}
      />

      <Modal
        title="Создание проекта"
        open={showCreate}
        onCancel={() => { setShowCreate(false); form.resetFields(); }}
        onOk={() => form.submit()}
        confirmLoading={createMutation.isPending}
        width={640}
        okText="Создать"
        cancelText="Отмена"
      >
        <Form form={form} layout="vertical" onFinish={createMutation.mutate}>
          <Form.Item name="name" label="Наименование проекта" rules={[{ required: true }]}>
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
          <Form.Item name="location" label="Местонахождение объекта" rules={[{ required: true }]}>
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
          <Form.Item name="director" label="Директор проекта" rules={[{ required: true }]}>
            <Input placeholder="Фамилия И.О." />
          </Form.Item>
          <Form.Item name="manager" label="Руководитель проекта" rules={[{ required: true }]}>
            <Input placeholder="Фамилия И.О." />
          </Form.Item>
          <Form.Item name="administrator" label="Администратор проекта" rules={[{ required: true }]}>
            <Input placeholder="Фамилия И.О." />
          </Form.Item>
          <Form.Item name="economist" label="Экономист проекта" rules={[{ required: true }]}>
            <Input placeholder="Фамилия И.О." />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
