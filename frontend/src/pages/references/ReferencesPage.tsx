import React, { useState } from 'react';
import {
  Tabs, Table, Button, Tag, Modal, Form, Input, Switch,
  message, Typography,
} from 'antd';
import { PlusOutlined, EditOutlined } from '@ant-design/icons';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import type { ColumnsType } from 'antd/es/table';
import { refsApi } from '../../api';
import type { Executor, Position, WorkMode } from '../../types';
import { hasRole } from '../../store/auth';
import { extractError } from '../../api/client';

const { Title } = Typography;
const canEdit = () => hasRole('GE');

// ─── Исполнители ────────────────────────────────────────────────────────────
function ExecutorsTab() {
  const qc = useQueryClient();
  const [showModal, setShowModal] = useState(false);
  const [editing, setEditing] = useState<Executor | null>(null);
  const [form] = Form.useForm();

  const { data, isLoading } = useQuery({
    queryKey: ['executors-all'],
    queryFn: () => refsApi.executors(),
  });

  const createMutation = useMutation({
    mutationFn: (vals: { name: string; full_name: string }) => refsApi.createExecutor(vals),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['executors-all'] });
      qc.invalidateQueries({ queryKey: ['executors'] });
      message.success('Исполнитель добавлен');
      setShowModal(false);
      form.resetFields();
    },
    onError: (e) => message.error(extractError(e)),
  });

  const updateMutation = useMutation({
    mutationFn: (vals: { name?: string; full_name?: string; active?: boolean }) =>
      refsApi.updateExecutor(editing!.id, vals),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['executors-all'] });
      qc.invalidateQueries({ queryKey: ['executors'] });
      message.success('Обновлено');
      setShowModal(false);
      form.resetFields();
      setEditing(null);
    },
    onError: (e) => message.error(extractError(e)),
  });

  const columns: ColumnsType<Executor> = [
    { title: 'Наименование (кратко)', dataIndex: 'name', sorter: (a, b) => a.name.localeCompare(b.name) },
    { title: 'Полное наименование', dataIndex: 'full_name' },
    {
      title: 'Статус', dataIndex: 'active',
      render: (v) => <Tag color={v ? 'green' : 'default'}>{v ? 'Активен' : 'Неактивен'}</Tag>,
    },
    ...(canEdit() ? [{
      title: '', key: 'edit', width: 60,
      render: (_: unknown, r: Executor) => (
        <Button size="small" icon={<EditOutlined />}
          onClick={() => { setEditing(r); form.setFieldsValue(r); setShowModal(true); }} />
      ),
    }] : []),
  ];

  return (
    <>
      {canEdit() && (
        <Button type="primary" icon={<PlusOutlined />} style={{ marginBottom: 12, background: '#1a3a6b' }}
          onClick={() => { setEditing(null); form.resetFields(); setShowModal(true); }}>
          Добавить исполнителя
        </Button>
      )}
      <Table rowKey="id" columns={columns} dataSource={data ?? []} loading={isLoading} size="small" pagination={false} />
      <Modal
        title={editing ? 'Редактирование исполнителя' : 'Новый исполнитель'}
        open={showModal}
        onCancel={() => { setShowModal(false); setEditing(null); form.resetFields(); }}
        onOk={() => form.submit()} okText="Сохранить" cancelText="Отмена"
        confirmLoading={createMutation.isPending || updateMutation.isPending}
      >
        <Form form={form} layout="vertical"
          onFinish={(vals) => editing ? updateMutation.mutate(vals) : createMutation.mutate(vals)}>
          <Form.Item name="name" label="Краткое наименование" rules={[{ required: true }]}>
            <Input placeholder="Айбикон" />
          </Form.Item>
          <Form.Item name="full_name" label="Полное наименование" rules={[{ required: !editing }]}>
            <Input placeholder='ООО «АйБиКон»' />
          </Form.Item>
          {editing && (
            <Form.Item name="active" label="Активен" valuePropName="checked">
              <Switch />
            </Form.Item>
          )}
        </Form>
      </Modal>
    </>
  );
}

// ─── Должности ──────────────────────────────────────────────────────────────
function PositionsTab() {
  const qc = useQueryClient();
  const [showModal, setShowModal] = useState(false);
  const [editing, setEditing] = useState<Position | null>(null);
  const [form] = Form.useForm();

  const { data, isLoading } = useQuery({ queryKey: ['positions-all'], queryFn: refsApi.positions });

  const createMutation = useMutation({
    mutationFn: (vals: { name: string }) => refsApi.createPosition(vals.name),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['positions-all'] });
      message.success('Должность добавлена');
      setShowModal(false);
      form.resetFields();
    },
    onError: (e) => message.error(extractError(e)),
  });

  const updateMutation = useMutation({
    mutationFn: (vals: { name?: string; active?: boolean }) =>
      refsApi.updatePosition(editing!.id, vals),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['positions-all'] });
      message.success('Обновлено');
      setShowModal(false);
      form.resetFields();
      setEditing(null);
    },
    onError: (e) => message.error(extractError(e)),
  });

  const columns: ColumnsType<Position> = [
    { title: 'Должность', dataIndex: 'name', sorter: (a, b) => a.name.localeCompare(b.name) },
    { title: 'Статус', dataIndex: 'active', render: (v) => <Tag color={v ? 'green' : 'default'}>{v ? 'Активна' : 'Неактивна'}</Tag> },
    ...(canEdit() ? [{ title: '', key: 'edit', width: 60, render: (_: unknown, r: Position) => (
      <Button size="small" icon={<EditOutlined />}
        onClick={() => { setEditing(r); form.setFieldsValue(r); setShowModal(true); }} />
    )}] : []),
  ];

  return (
    <>
      {canEdit() && (
        <Button type="primary" icon={<PlusOutlined />} style={{ marginBottom: 12, background: '#1a3a6b' }}
          onClick={() => { setEditing(null); form.resetFields(); setShowModal(true); }}>
          Добавить должность
        </Button>
      )}
      <Table rowKey="id" columns={columns} dataSource={data ?? []} loading={isLoading} size="small" pagination={{ pageSize: 25 }} />
      <Modal
        title={editing ? 'Редактирование должности' : 'Новая должность'}
        open={showModal}
        onCancel={() => { setShowModal(false); setEditing(null); form.resetFields(); }}
        onOk={() => form.submit()} okText="Сохранить" cancelText="Отмена"
        confirmLoading={createMutation.isPending || updateMutation.isPending}
      >
        <Form form={form} layout="vertical"
          onFinish={(vals) => editing ? updateMutation.mutate(vals) : createMutation.mutate(vals)}>
          <Form.Item name="name" label="Наименование" rules={[{ required: true }]}><Input /></Form.Item>
          {editing && <Form.Item name="active" label="Активна" valuePropName="checked"><Switch /></Form.Item>}
        </Form>
      </Modal>
    </>
  );
}

// ─── Режимы работы ──────────────────────────────────────────────────────────
function WorkModesTab() {
  const qc = useQueryClient();
  const [showModal, setShowModal] = useState(false);
  const [editing, setEditing] = useState<WorkMode | null>(null);
  const [form] = Form.useForm();

  const { data, isLoading } = useQuery({ queryKey: ['work-modes-all'], queryFn: refsApi.workModes });

  const createMutation = useMutation({
    mutationFn: (vals: { code: string; full_name: string }) => refsApi.createWorkMode(vals),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['work-modes-all'] });
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
      qc.invalidateQueries({ queryKey: ['work-modes-all'] });
      message.success('Обновлено');
      setShowModal(false);
      form.resetFields();
      setEditing(null);
    },
    onError: (e) => message.error(extractError(e)),
  });

  const columns: ColumnsType<WorkMode> = [
    { title: 'Код', dataIndex: 'code', width: 100 },
    { title: 'Полное наименование', dataIndex: 'full_name', sorter: (a, b) => a.full_name.localeCompare(b.full_name) },
    { title: 'Статус', dataIndex: 'active', render: (v) => <Tag color={v ? 'green' : 'default'}>{v ? 'Активен' : 'Неактивен'}</Tag> },
    ...(canEdit() ? [{ title: '', key: 'edit', width: 60, render: (_: unknown, r: WorkMode) => (
      <Button size="small" icon={<EditOutlined />}
        onClick={() => { setEditing(r); form.setFieldsValue(r); setShowModal(true); }} />
    )}] : []),
  ];

  return (
    <>
      {canEdit() && (
        <Button type="primary" icon={<PlusOutlined />} style={{ marginBottom: 12, background: '#1a3a6b' }}
          onClick={() => { setEditing(null); form.resetFields(); setShowModal(true); }}>
          Добавить режим работы
        </Button>
      )}
      <Table rowKey="id" columns={columns} dataSource={data ?? []} loading={isLoading} size="small" pagination={false} />
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
          {editing && <Form.Item name="active" label="Активен" valuePropName="checked"><Switch /></Form.Item>}
        </Form>
      </Modal>
    </>
  );
}

export default function ReferencesPage() {
  return (
    <div>
      <Title level={4} style={{ marginBottom: 16 }}>Справочники</Title>
      <Tabs
        items={[
          { key: 'executors', label: 'Исполнители', children: <ExecutorsTab /> },
          { key: 'positions', label: 'Должности', children: <PositionsTab /> },
          { key: 'work-modes', label: 'Режимы работы', children: <WorkModesTab /> },
        ]}
      />
    </div>
  );
}
