import React, { useCallback, useEffect, useRef, useState } from 'react';
import {
  Tabs, Table, Button, Modal, Form, Input, Switch,
  message,
} from 'antd';
import { PlusOutlined, EditOutlined } from '@ant-design/icons';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import type { ColumnsType } from 'antd/es/table';
import { refsApi } from '../../api';
import type { Executor, Position, WorkMode } from '../../types';
import { hasRole } from '../../store/auth';
import { extractError } from '../../api/client';
import StatusTag from '../../components/StatusTag';

const canEdit = () => hasRole('GE');

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

// ─── Исполнители ────────────────────────────────────────────────────────────
function ExecutorsTab({ addSignal }: TabProps) {
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
      render: (v) => (
        <StatusTag color={v ? 'green' : 'grey'}>
          {v ? 'Активен' : 'Неактивен'}
        </StatusTag>
      ),
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
      <Table rowKey="id" columns={columns} className="nowrap-table" scroll={{ x: 'max-content' }} dataSource={data ?? []} loading={isLoading} size="small" pagination={false} />
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
function PositionsTab({ addSignal }: TabProps) {
  const qc = useQueryClient();
  const [showModal, setShowModal] = useState(false);
  const [editing, setEditing] = useState<Position | null>(null);
  const [form] = Form.useForm();

  const openAdd = useCallback(() => {
    setEditing(null);
    form.resetFields();
    setShowModal(true);
  }, [form]);
  useAddSignal(addSignal, openAdd);

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
    { title: 'Статус', dataIndex: 'active',
      render: (v) => (
        <StatusTag color={v ? 'green' : 'grey'}>{v ? 'Активна' : 'Неактивна'}</StatusTag>
      ) },
    ...(canEdit() ? [{ title: '', key: 'edit', width: 60, render: (_: unknown, r: Position) => (
      <Button size="small" icon={<EditOutlined />}
        onClick={() => { setEditing(r); form.setFieldsValue(r); setShowModal(true); }} />
    )}] : []),
  ];

  return (
    <>
      <Table rowKey="id" columns={columns} className="nowrap-table" scroll={{ x: 'max-content' }} dataSource={data ?? []} loading={isLoading} size="small" pagination={{ pageSize: 25 }} />
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
function WorkModesTab({ addSignal }: TabProps) {
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
    { title: 'Статус', dataIndex: 'active',
      render: (v) => (
        <StatusTag color={v ? 'green' : 'grey'}>{v ? 'Активен' : 'Неактивен'}</StatusTag>
      ) },
    ...(canEdit() ? [{ title: '', key: 'edit', width: 60, render: (_: unknown, r: WorkMode) => (
      <Button size="small" icon={<EditOutlined />}
        onClick={() => { setEditing(r); form.setFieldsValue(r); setShowModal(true); }} />
    )}] : []),
  ];

  return (
    <>
      <Table rowKey="id" columns={columns} className="nowrap-table" scroll={{ x: 'max-content' }} dataSource={data ?? []} loading={isLoading} size="small" pagination={false} />
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
  const [tab, setTab] = useState('executors');
  const [addSignal, setAddSignal] = useState(0);

  const ADD_LABELS: Record<string, string> = {
    executors: 'Добавить исполнителя',
    positions: 'Добавить должность',
    'work-modes': 'Добавить режим работы',
  };

  return (
    <div>
      {/* Название раздела живёт в шапке (AppLayout). */}
      <Tabs
        activeKey={tab}
        onChange={setTab}
        // Неактивные вкладки размонтируются: тогда сигнал «добавить»
        // получает ровно одна вкладка — та, что на экране.
        destroyOnHidden
        tabBarExtraContent={canEdit() && (
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => setAddSignal(s => s + 1)}
          >
            {ADD_LABELS[tab]}
          </Button>
        )}
        items={[
          { key: 'executors', label: 'Исполнители', children: <ExecutorsTab addSignal={addSignal} /> },
          { key: 'positions', label: 'Должности', children: <PositionsTab addSignal={addSignal} /> },
          { key: 'work-modes', label: 'Режимы работы', children: <WorkModesTab addSignal={addSignal} /> },
        ]}
      />
    </div>
  );
}
