import React, { useState } from 'react';
import {
  Card, Descriptions, Tag, Button, Space, Modal, Form, Select,
  Input, Table, Typography, message, Tooltip, DatePicker, InputNumber,
  Popconfirm,
} from 'antd';
import {
  EditOutlined, PlusOutlined, ArrowLeftOutlined, DownloadOutlined,
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
import { fmtDate, fmtMoney } from '../../utils/fmt';
import Profitability from '../../components/Profitability';
import { hasRole } from '../../store/auth';
import { shortName } from '../../utils/names';
import { extractError } from '../../api/client';

const { Title, Text } = Typography;

/** «Согласовано от ДД.ММ.ГГГГ. » — обязательное начало комментария. */
function approvalPrefixText(): string {
  const d = new Date();
  const dd = String(d.getDate()).padStart(2, '0');
  const mm = String(d.getMonth() + 1).padStart(2, '0');
  return `Согласовано от ${dd}.${mm}.${d.getFullYear()}. `;
}

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
  const [versionStatusForm] = Form.useForm();
  const [versionStatusTarget, setVersionStatusTarget] =
    useState<{ version: BudgetVersion; status: string } | null>(null);
  const [versionApprovalPrefix, setVersionApprovalPrefix] = useState('');

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

  const versionStatusMutation = useMutation({
    mutationFn: ({ vid, status, comment }: { vid: number; status: string; comment: string }) =>
      budgetsApi.changeStatus(vid, status, comment),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['budget-versions', pid] });
      qc.invalidateQueries({ queryKey: ['project', pid] });
      qc.invalidateQueries({ queryKey: ['projects'] });
      message.success('Статус бюджета изменён');
      setVersionStatusTarget(null);
      versionStatusForm.resetFields();
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
  const canChangeBudgetStatus = hasRole('GE', 'EP');

  /**
   * Смена статуса версии прямо из таблицы. Причина обязательна, а при
   * согласовании комментарий начинается с неудаляемой даты — правила те
   * же, что и на экране самой версии.
   */
  function openVersionStatus(v: BudgetVersion, status: string) {
    setVersionStatusTarget({ version: v, status });
    const prefix = status === 'approved' ? approvalPrefixText() : '';
    setVersionApprovalPrefix(prefix);
    versionStatusForm.setFieldsValue({ comment: prefix });
  }

  const validNextStatuses: Record<string, string[]> = {
    prospect: ['active', 'suspended', 'completed', 'unrealized'],
    active: ['prospect', 'suspended', 'completed', 'unrealized'],
    suspended: ['prospect', 'active', 'completed', 'unrealized'],
    completed: ['prospect', 'active'],
    unrealized: ['prospect', 'active'],
  };

  /** Статусы, в которые можно перевести версию из таблицы. */
  const validBudgetNext: Record<string, string[]> = {
    draft: ['under_review'],
    under_review: ['approved', 'draft'],
    approved: [],
    archive: [],
  };

  const versionColumns: ColumnsType<BudgetVersion> = [
    {
      // ТЗ, таблица 4: у проекта с ID 1 бюджеты нумеруются 1.1, 1.2 …
      title: 'ID бюджета',
      key: 'budget_code',
      width: 110,
      render: (_, r) => <Text strong>{pid}.{r.version_no}</Text>,
    },
    {
      title: 'Версия',
      dataIndex: 'version_no',
      width: 80,
      render: (n: number) => n,
    },
    {
      title: 'Дата создания',
      dataIndex: 'created_at',
      width: 130,
      render: fmtDate,
    },
    {
      title: 'Автор',
      dataIndex: 'created_by_name',
      width: 160,
      render: (n: string) => shortName(n),
    },
    {
      title: 'Статус бюджета',
      dataIndex: 'status',
      width: 190,
      render: (st: string, r) => {
        const next = validBudgetNext[st] ?? [];
        // Менять статус прямо из таблицы могут те же роли, что и на
        // экране бюджета; в конечных статусах менять нечего.
        if (!canChangeBudgetStatus || next.length === 0) {
          return <Tag color={BUDGET_STATUS_COLORS[st]}>{BUDGET_STATUS_LABELS[st] ?? st}</Tag>;
        }
        return (
          <Select
            size="small"
            style={{ width: 170 }}
            value={st}
            onChange={(v) => openVersionStatus(r, v)}
            options={[
              { value: st, label: BUDGET_STATUS_LABELS[st] ?? st, disabled: true },
              ...next.map(v => ({ value: v, label: BUDGET_STATUS_LABELS[v] ?? v })),
            ]}
          />
        );
      },
    },
    {
      title: 'Стоимость без НДС',
      dataIndex: 'cost_no_vat',
      width: 160,
      render: fmtMoney,
      align: 'right',
    },
    {
      title: 'Рентабельность, %',
      dataIndex: 'profitability',
      width: 150,
      render: (v: number | null | undefined) => <Profitability value={v} />,
      align: 'right',
    },
    {
      title: 'Комментарий',
      dataIndex: 'comment',
      ellipsis: true,
    },
    {
      title: '',
      key: 'open',
      width: 100,
      render: (_, r) => (
        <Button size="small" onClick={() => navigate(`/budget-versions/${r.id}`)}>
          Открыть
        </Button>
      ),
    },
    {
      title: '',
      key: 'download',
      width: 110,
      render: (_, r) => (
        <Tooltip title="Выгрузка XLSX появится вместе с формированием БДР/БДДС">
          <Button size="small" icon={<DownloadOutlined />} disabled>
            Скачать
          </Button>
        </Tooltip>
      ),
    },
  ];

  /**
   * Порядок вывода версий: Согласован → На согласовании → Черновик →
   * Архив, внутри статуса — новые сверху.
   */
  const STATUS_ORDER: Record<string, number> = {
    approved: 0,
    under_review: 1,
    draft: 2,
    archive: 3,
  };
  const sortedVersions = [...(versions ?? [])].sort((a, b) => {
    const d = (STATUS_ORDER[a.status] ?? 9) - (STATUS_ORDER[b.status] ?? 9);
    if (d !== 0) return d;
    return b.created_at.localeCompare(a.created_at);
  });

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
          <Descriptions.Item label="Автор">{shortName(project.created_by_name)}</Descriptions.Item>
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
          dataSource={sortedVersions}
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

      {/* Смена статуса версии бюджета прямо из таблицы */}
      <Modal
        title={versionStatusTarget
          ? `Версия ${pid}.${versionStatusTarget.version.version_no} → «${BUDGET_STATUS_LABELS[versionStatusTarget.status]}»`
          : ''}
        open={!!versionStatusTarget}
        onCancel={() => { setVersionStatusTarget(null); versionStatusForm.resetFields(); }}
        onOk={() => versionStatusForm.submit()}
        confirmLoading={versionStatusMutation.isPending}
        okText="Изменить"
        cancelText="Отмена"
        destroyOnClose
      >
        <Form
          form={versionStatusForm}
          layout="vertical"
          onFinish={(vals: { comment: string }) => versionStatusTarget && versionStatusMutation.mutate({
            vid: versionStatusTarget.version.id,
            status: versionStatusTarget.status,
            comment: vals.comment,
          })}
        >
          <Form.Item
            name="comment"
            label="Причина изменения"
            rules={[
              { required: true, message: 'Укажите причину изменения статуса' },
              {
                validator: (_, value: string) =>
                  !versionApprovalPrefix || (value ?? '').startsWith(versionApprovalPrefix)
                    ? Promise.resolve()
                    : Promise.reject(new Error(
                      `Комментарий должен начинаться с «${versionApprovalPrefix.trim()}»`)),
              },
            ]}
            extra={versionApprovalPrefix
              ? 'Дата согласования подставлена автоматически и не удаляется.'
              : undefined}
          >
            <Input.TextArea
              rows={3}
              onChange={(e) => {
                if (versionApprovalPrefix && !e.target.value.startsWith(versionApprovalPrefix)) {
                  versionStatusForm.setFieldValue('comment', versionApprovalPrefix);
                }
              }}
            />
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
