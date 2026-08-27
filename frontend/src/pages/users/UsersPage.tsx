import React, { useState } from 'react';
import {
  Table, Button, Space, Modal, Form, Input, Select,
  message,
} from 'antd';
import {
  PlusOutlined, EditOutlined, KeyOutlined, UnlockOutlined, UserAddOutlined,
} from '@ant-design/icons';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import type { ColumnsType } from 'antd/es/table';
import { usersApi, projectsApi } from '../../api';
import type { User } from '../../types';
import { ROLES, ROLE_LABELS } from '../../types';
import { extractError } from '../../api/client';
import StatusTag from '../../components/StatusTag';


export default function UsersPage() {
  const qc = useQueryClient();
  const [showCreate, setShowCreate] = useState(false);
  const [showEdit, setShowEdit] = useState(false);
  const [showPwd, setShowPwd] = useState(false);
  const [showAccess, setShowAccess] = useState(false);
  const [selected, setSelected] = useState<User | null>(null);
  const [createForm] = Form.useForm();
  const [editForm] = Form.useForm();
  const [pwdForm] = Form.useForm();
  const [accessForm] = Form.useForm();

  const { data: users, isLoading } = useQuery({
    queryKey: ['users'],
    queryFn: usersApi.list,
  });

  const { data: projectsData } = useQuery({
    queryKey: ['projects-all'],
    queryFn: () => projectsApi.list({ limit: 200 }),
  });

  const createMutation = useMutation({
    mutationFn: (vals: { email: string; full_name: string; role: string; password: string }) =>
      usersApi.create(vals),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['users'] });
      message.success('Пользователь создан');
      setShowCreate(false);
      createForm.resetFields();
    },
    onError: (e) => message.error(extractError(e)),
  });

  const updateMutation = useMutation({
    mutationFn: (vals: { full_name?: string; role?: string; active?: boolean }) =>
      usersApi.update(selected!.id, vals),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['users'] });
      message.success('Сохранено');
      setShowEdit(false);
      editForm.resetFields();
      setSelected(null);
    },
    onError: (e) => message.error(extractError(e)),
  });

  const pwdMutation = useMutation({
    mutationFn: (vals: { password: string }) => usersApi.setPassword(selected!.id, vals.password),
    onSuccess: () => {
      message.success('Пароль изменён');
      setShowPwd(false);
      pwdForm.resetFields();
      setSelected(null);
    },
    onError: (e) => message.error(extractError(e)),
  });

  const unlockMutation = useMutation({
    mutationFn: (id: number) => usersApi.unlock(id),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['users'] }); message.success('Разблокировано'); },
    onError: (e) => message.error(extractError(e)),
  });

  const grantMutation = useMutation({
    mutationFn: (vals: { project_id: number }) =>
      usersApi.grantAccess(selected!.id, vals.project_id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['users'] });
      message.success('Доступ выдан');
      setShowAccess(false);
      accessForm.resetFields();
    },
    onError: (e) => message.error(extractError(e)),
  });

  const roleOptions = Object.values(ROLES).map((r) => ({ value: r, label: ROLE_LABELS[r] }));

  const columns: ColumnsType<User> = [
    // В управлении пользователями ФИО показывается ПОЛНОСТЬЮ
    { title: 'ФИО', dataIndex: 'full_name', sorter: (a, b) => a.full_name.localeCompare(b.full_name, 'ru') },
    { title: 'Email', dataIndex: 'email' },
    {
      title: 'Роль',
      dataIndex: 'role',
      render: (r) => <StatusTag>{ROLE_LABELS[r as keyof typeof ROLE_LABELS] ?? r}</StatusTag>,
    },
    {
      title: 'Статус',
      dataIndex: 'active',
      render: (active: boolean, row: User) => (
        <Space>
          <StatusTag color={active ? 'green' : 'red'}>
            {active ? 'Активен' : 'Заблокирован'}
          </StatusTag>
          {row.failed_attempts >= 5 && (
            <StatusTag color="amber">Много попыток входа</StatusTag>
          )}
        </Space>
      ),
    },
    {
      title: '',
      key: 'actions',
      width: 180,
      render: (_, row) => (
        <Space size="small">
          <Button
            size="small" icon={<EditOutlined />} title="Редактировать"
            onClick={() => { setSelected(row); editForm.setFieldsValue({ full_name: row.full_name, email: row.email, role: row.role }); setShowEdit(true); }}
          />
          <Button
            size="small" icon={<KeyOutlined />} title="Сменить пароль"
            onClick={() => { setSelected(row); setShowPwd(true); }}
          />
          <Button
            size="small" icon={<UserAddOutlined />} title="Выдать доступ к проекту"
            onClick={() => { setSelected(row); setShowAccess(true); }}
          />
          {row.failed_attempts >= 5 && (
            <Button
              size="small" icon={<UnlockOutlined />} title="Разблокировать"
              onClick={() => unlockMutation.mutate(row.id)}
              loading={unlockMutation.isPending}
            />
          )}
        </Space>
      ),
    },
  ];

  const projectOptions = (projectsData?.items ?? []).map((p) => ({
    value: p.id,
    label: `#${p.id} ${p.name}`,
  }));

  return (
    <div>
      {/* Название раздела живёт в шапке (AppLayout). */}
      <Button
        type="primary" icon={<PlusOutlined />} style={{ marginBottom: 12 }}
        onClick={() => { setShowCreate(true); createForm.resetFields(); }}
      >
        Создать пользователя
      </Button>

      <Table
        rowKey="id"
        className="nowrap-table"
        columns={columns}
        dataSource={users ?? []}
        loading={isLoading}
        size="small"
        pagination={{ pageSize: 20 }}
        scroll={{ x: 'max-content' }}
      />

      {/* Создать */}
      <Modal
        title="Новый пользователь" open={showCreate}
        onCancel={() => { setShowCreate(false); createForm.resetFields(); }}
        onOk={() => createForm.submit()} okText="Создать" cancelText="Отмена"
        confirmLoading={createMutation.isPending}
      >
        <Form form={createForm} layout="vertical" onFinish={createMutation.mutate}>
          <Form.Item
            name="full_name"
            label="ФИО полностью"
            rules={[{ required: true, message: 'Не заполнено обязательное поле: ФИО' }]}
            extra="Фамилия Имя Отчество. В таблицах и шапке показывается сокращённо: Фамилия И.О."
          >
            <Input placeholder="Иванов Иван Иванович" />
          </Form.Item>
          <Form.Item name="email" label="Email" rules={[{ required: true }, { type: 'email' }]}><Input /></Form.Item>
          <Form.Item name="role" label="Роль" rules={[{ required: true }]}>
            <Select options={roleOptions} />
          </Form.Item>
          <Form.Item name="password" label="Пароль" rules={[{ required: true, min: 10, message: 'Минимум 10 символов' }]}>
            <Input.Password />
          </Form.Item>
        </Form>
      </Modal>

      {/* Редактировать */}
      <Modal
        title="Редактирование" open={showEdit}
        onCancel={() => { setShowEdit(false); setSelected(null); }}
        onOk={() => editForm.submit()} okText="Сохранить" cancelText="Отмена"
        confirmLoading={updateMutation.isPending}
      >
        <Form form={editForm} layout="vertical" onFinish={updateMutation.mutate}>
          <Form.Item
            name="full_name"
            label="ФИО полностью"
            rules={[{ required: true, message: 'Не заполнено обязательное поле: ФИО' }]}
            extra="Фамилия Имя Отчество. В таблицах и шапке показывается сокращённо: Фамилия И.О."
          >
            <Input placeholder="Иванов Иван Иванович" />
          </Form.Item>
          <Form.Item name="role" label="Роль" rules={[{ required: true }]}>
            <Select options={roleOptions} />
          </Form.Item>
        </Form>
      </Modal>

      {/* Смена пароля */}
      <Modal
        title={`Смена пароля — ${selected?.full_name ?? ''}`} open={showPwd}
        onCancel={() => { setShowPwd(false); setSelected(null); pwdForm.resetFields(); }}
        onOk={() => pwdForm.submit()} okText="Сменить" cancelText="Отмена"
        confirmLoading={pwdMutation.isPending}
      >
        <Form form={pwdForm} layout="vertical" onFinish={pwdMutation.mutate}>
          <Form.Item name="password" label="Новый пароль" rules={[{ required: true, min: 10, message: 'Минимум 10 символов' }]}>
            <Input.Password />
          </Form.Item>
        </Form>
      </Modal>

      {/* Доступ к проекту */}
      <Modal
        title={`Доступ к проекту — ${selected?.full_name ?? ''}`} open={showAccess}
        onCancel={() => { setShowAccess(false); setSelected(null); accessForm.resetFields(); }}
        onOk={() => accessForm.submit()} okText="Выдать" cancelText="Отмена"
        confirmLoading={grantMutation.isPending}
      >
        <Form form={accessForm} layout="vertical" onFinish={grantMutation.mutate}>
          <Form.Item name="project_id" label="Проект" rules={[{ required: true }]}>
            <Select
              options={projectOptions}
              showSearch
              filterOption={(inp, opt) => String(opt?.label).toLowerCase().includes(inp.toLowerCase())}
            />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
