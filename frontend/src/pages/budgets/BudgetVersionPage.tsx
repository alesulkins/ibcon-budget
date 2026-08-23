import React, { useState } from 'react';
import {
  Card, Steps, Button, Space, Tag, Typography, Spin, message,
  Modal, Form, Input, Select, Tabs, Statistic, Row, Col, Alert,
} from 'antd';
import {
  ArrowLeftOutlined, CalculatorOutlined, FileExcelOutlined,
} from '@ant-design/icons';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useParams, useNavigate } from 'react-router-dom';
import { budgetsApi, projectsApi } from '../../api';
import { BUDGET_STATUS_LABELS, BUDGET_STATUS_COLORS } from '../../types';
import { hasRole } from '../../store/auth';
import { fmtMoney, fmtPct, fmtDate } from '../../utils/fmt';
import { extractError } from '../../api/client';
import EmployeesInput from './inputs/EmployeesInput';
import BonusesInput from './inputs/BonusesInput';
import RentApartmentsInput from './inputs/RentApartmentsInput';
import SimpleCostInput from './inputs/SimpleCostInput';
import OverheadInput from './inputs/OverheadInput';
import BudgetParamsInput from './inputs/BudgetParamsInput';
import CalcResults from './CalcResults';

const { Title, Text } = Typography;

// Шаги визарда (соответствуют листам 4.1-4.12 + накладные + параметры + результаты)
const WIZARD_STEPS = [
  { key: 'employees',    title: 'Сотрудники',              desc: '4.6 – ФОТ и билеты' },
  { key: 'bonuses',      title: 'Премии',                  desc: '4.1 – Премии и компенсации' },
  { key: 'rent_apartments_realtor', title: 'Аренда жилья', desc: '4.2 – Квартиры и риелтор' },
  { key: 'transport_garage', title: 'Транспорт',           desc: '4.3 – Аренда и гараж' },
  { key: 'site_setup',   title: 'Стройплощадка',           desc: '4.4 – Обустройство' },
  { key: 'office',       title: 'Офис',                    desc: '4.5 – Аренда и уборка' },
  { key: 'control_equipment', title: 'СК оборудование',    desc: '4.7 – Приборы' },
  { key: 'software',     title: 'ПО',                      desc: '4.8 – Программное обеспечение' },
  { key: 'subcontract_ext', title: 'Субподряд ГПХ внешний', desc: '4.9' },
  { key: 'subcontract_emp', title: 'Субподряд ГПХ сотр.',  desc: '4.10' },
  { key: 'subcontract_gen', title: 'Субподряд',            desc: '4.11' },
  { key: 'corporate_events', title: 'Корпоративы',         desc: '4.12 – Корпоративные мероприятия' },
  { key: 'overhead',     title: 'Прочие расходы',          desc: 'Строки 178-211 (накладные)' },
  { key: 'params',       title: 'Параметры',               desc: 'Непредвиденные, АУП, БГ, маржа' },
  { key: 'results',      title: 'Результаты',              desc: 'Итоги расчёта бюджета' },
];

export default function BudgetVersionPage() {
  const { vid } = useParams<{ vid: string }>();
  const versionId = Number(vid);
  const navigate = useNavigate();
  const qc = useQueryClient();
  const [step, setStep] = useState(0);
  const [statusForm] = Form.useForm();
  const [showStatus, setShowStatus] = useState(false);

  const { data: version, isLoading } = useQuery({
    queryKey: ['budget-version', versionId],
    queryFn: () => budgetsApi.getVersion(versionId),
  });

  const { data: project } = useQuery({
    queryKey: ['project', version?.project_id],
    queryFn: () => projectsApi.get(version!.project_id),
    enabled: !!version?.project_id,
  });

  const statusMutation = useMutation({
    mutationFn: ({ status, comment }: { status: string; comment: string }) =>
      budgetsApi.changeStatus(versionId, status, comment),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['budget-version', versionId] });
      qc.invalidateQueries({ queryKey: ['budget-versions', version?.project_id] });
      message.success('Статус бюджета изменён');
      setShowStatus(false);
    },
    onError: (e) => message.error(extractError(e)),
  });

  // АП видит только строки 178-214 (шаг overhead)
  const isAP = hasRole('AP');
  const apOnlyStepIdx = WIZARD_STEPS.findIndex(s => s.key === 'overhead');

  const canEdit = hasRole('GE', 'EP', 'IP') && !['approved', 'archive'].includes(version?.status ?? '');
  const canChangeStatus = hasRole('GE', 'EP');

  const validNextStatuses: Record<string, string[]> = {
    draft: ['under_review'],
    under_review: ['approved', 'draft'],
    approved: [],
    archive: [],
  };

  if (isLoading || !version || !project) return <Spin size="large" />;

  const isReadonly = ['approved', 'archive'].includes(version.status);

  function renderStepContent() {
    const duration = project!.duration_months;

    switch (WIZARD_STEPS[step].key) {
      case 'employees':
        return (
          <EmployeesInput
            versionId={versionId}
            duration={duration}
            startDate={project!.start_date}
            readonly={isReadonly}
          />
        );
      case 'bonuses':
        return (
          <BonusesInput
            versionId={versionId}
            duration={duration}
            startDate={project!.start_date}
            readonly={isReadonly}
          />
        );
      case 'rent_apartments_realtor':
        // Количество квартир и цены; аренду и риелтора считает бэкенд (4.2).
        return (
          <RentApartmentsInput
            versionId={versionId}
            duration={duration}
            startDate={project!.start_date}
            readonly={isReadonly}
          />
        );
      case 'transport_garage':
        return (
          <div>
            <Alert
              type="info"
              showIcon
              style={{ marginBottom: 12 }}
              message='Строка "Аренда транспорта" в бюджете включает как ежемесячную аренду, так и разовые покупки авто. Если в каком-то месяце планируется покупка — добавьте её стоимость к аренде за этот месяц.'
            />
            <SimpleCostInput
              versionId={versionId}
              type="transport_rental"
              title="Аренда авто (включая покупку) — лист 4.3"
              duration={duration}
              startDate={project!.start_date}
              readonly={isReadonly}
            />
            <div style={{ marginTop: 16 }}>
              <SimpleCostInput
                versionId={versionId}
                type="garage_rent"
                title="Аренда гаража — лист 4.3"
                duration={duration}
                startDate={project!.start_date}
                readonly={isReadonly}
              />
            </div>
          </div>
        );
      case 'site_setup':
        return (
          <div>
            <Alert
              type="info"
              showIcon
              style={{ marginBottom: 12 }}
              message='Строка "Обустройство стройплощадки" в бюджете включает аренду и покупку вагончиков. Если в каком-то месяце планируется покупка — добавьте её стоимость к сумме за этот месяц.'
            />
            <SimpleCostInput
              versionId={versionId}
              type="trailer_rent"
              title="Аренда/обустройство вагончиков (включая покупку) — лист 4.4"
              duration={duration}
              startDate={project!.start_date}
              readonly={isReadonly}
            />
          </div>
        );
      case 'office':
        return (
          <div>
            <SimpleCostInput
              versionId={versionId}
              type="office_rent"
              title="Аренда офиса — лист 4.5"
              duration={duration}
              startDate={project!.start_date}
              readonly={isReadonly}
            />
            <div style={{ marginTop: 16 }}>
              <SimpleCostInput
                versionId={versionId}
                type="office_cleaning"
                title="Уборка офиса — лист 4.5"
                duration={duration}
                startDate={project!.start_date}
                readonly={isReadonly}
              />
            </div>
          </div>
        );
      case 'control_equipment':
        return (
          <SimpleCostInput
            versionId={versionId}
            type="control_equipment"
            title="Приобретение приборов стройконтроля — лист 4.7"
            duration={duration}
            startDate={project!.start_date}
            readonly={isReadonly}
          />
        );
      case 'software':
        return (
          <SimpleCostInput
            versionId={versionId}
            type="software"
            title="ПО, лицензии — лист 4.8"
            duration={duration}
            startDate={project!.start_date}
            readonly={isReadonly}
          />
        );
      case 'subcontract_ext':
        return (
          <SimpleCostInput
            versionId={versionId}
            type="subcontract_ext"
            title="ГПХ внешний (контрагенты/услуги) — лист 4.9"
            duration={duration}
            startDate={project!.start_date}
            readonly={isReadonly}
          />
        );
      case 'subcontract_emp':
        return (
          <SimpleCostInput
            versionId={versionId}
            type="subcontract_emp"
            title="ГПХ сотрудников — лист 4.10"
            duration={duration}
            startDate={project!.start_date}
            readonly={isReadonly}
          />
        );
      case 'subcontract_gen':
        return (
          <SimpleCostInput
            versionId={versionId}
            type="subcontract_gen"
            title="Субподрядные работы — лист 4.11"
            duration={duration}
            startDate={project!.start_date}
            readonly={isReadonly}
          />
        );
      case 'corporate_events':
        return (
          <SimpleCostInput
            versionId={versionId}
            type="corporate_events"
            title="Корпоративные мероприятия — лист 4.12"
            duration={duration}
            startDate={project!.start_date}
            readonly={isReadonly}
          />
        );
      case 'overhead':
        return (
          <OverheadInput
            versionId={versionId}
            duration={duration}
            readonly={isReadonly}
          />
        );
      case 'params':
        return (
          <BudgetParamsInput
            versionId={versionId}
            readonly={isReadonly}
          />
        );
      case 'results':
        return (
          <CalcResults
            versionId={versionId}
            projectId={version!.project_id}
            duration={duration}
            startDate={project!.start_date}
          />
        );
      default:
        return null;
    }
  }

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button
          icon={<ArrowLeftOutlined />}
          onClick={() => navigate(`/projects/${version.project_id}`)}
        >
          К проекту
        </Button>
      </Space>

      <Card
        title={
          <Space>
            <Title level={4} style={{ margin: 0 }}>
              {project.name}
              {version.version_label ? ` — ${version.version_label}` : ''}
            </Title>
            <Tag color={BUDGET_STATUS_COLORS[version.status]}>
              {BUDGET_STATUS_LABELS[version.status]}
            </Tag>
          </Space>
        }
        extra={
          <Space>
            {canChangeStatus && (validNextStatuses[version.status] ?? []).length > 0 && (
              <Button onClick={() => setShowStatus(true)}>
                Изменить статус
              </Button>
            )}
          </Space>
        }
        style={{ marginBottom: 16 }}
      >
        <Row gutter={32}>
          <Col>
            <Text type="secondary">Создан: </Text>
            <Text>{fmtDate(version.created_at)} ({version.created_by_name})</Text>
          </Col>
          {version.cost_no_vat && (
            <Col>
              <Text type="secondary">Стоимость без НДС: </Text>
              <Text strong>{fmtMoney(version.cost_no_vat)}</Text>
            </Col>
          )}
          {version.profitability != null && (
            <Col>
              <Text type="secondary">Рентабельность: </Text>
              <Text strong>{fmtPct(version.profitability)}</Text>
            </Col>
          )}
        </Row>
        {version.comment && (
          <div style={{ marginTop: 8 }}>
            <Text type="secondary">Комментарий: </Text>
            <Text>{version.comment}</Text>
          </div>
        )}
        {isReadonly && (
          <Alert
            style={{ marginTop: 12 }}
            type="info"
            message="Версия доступна только для просмотра (согласована или архивная)"
            showIcon
          />
        )}
      </Card>

      {/* АП: только прочие расходы */}
      {isAP && (
        <Alert
          type="warning"
          message="Роль Администратора проекта: доступен только раздел «Прочие расходы» (строки 178–214)"
          showIcon style={{ marginBottom: 16 }}
        />
      )}

      {/* Навигационные вкладки по шагам */}
      <Card bodyStyle={{ padding: 0 }}>
        <div style={{ padding: '16px 24px', borderBottom: '1px solid #f0f0f0' }}>
          <Steps
            current={isAP ? 0 : step}
            onChange={isAP ? undefined : setStep}
            size="small"
            items={(isAP ? [WIZARD_STEPS[apOnlyStepIdx]] : WIZARD_STEPS).map((s) => ({
              title: s.title,
              description: s.desc,
            }))}
            style={{ overflowX: 'auto' }}
          />
        </div>
        <div style={{ padding: 24 }}>
          {isAP
            ? <OverheadInput versionId={versionId} duration={project.duration_months} readonly />
            : renderStepContent()}
        </div>
        <div style={{
          padding: '16px 24px',
          borderTop: '1px solid #f0f0f0',
          display: 'flex',
          justifyContent: 'space-between',
        }}>
          <Button
            disabled={step === 0 || isAP}
            onClick={() => setStep(s => s - 1)}
          >
            ← Назад
          </Button>
          <Button
            type="primary"
            disabled={step === WIZARD_STEPS.length - 1 || isAP}
            onClick={() => setStep(s => s + 1)}
            style={{ background: '#1a3a6b' }}
          >
            Далее →
          </Button>
        </div>
      </Card>

      {/* Модал изменения статуса */}
      <Modal
        title="Изменение статуса бюджета"
        open={showStatus}
        onCancel={() => setShowStatus(false)}
        onOk={() => statusForm.submit()}
        confirmLoading={statusMutation.isPending}
        okText="Изменить"
        cancelText="Отмена"
      >
        <Form
          form={statusForm}
          layout="vertical"
          onFinish={statusMutation.mutate}
        >
          <Form.Item name="status" label="Новый статус" rules={[{ required: true }]}>
            <Select
              options={(validNextStatuses[version.status] ?? []).map(s => ({
                value: s,
                label: BUDGET_STATUS_LABELS[s] ?? s,
              }))}
              onChange={(v) => {
                if (v === 'approved') {
                  const today = new Date();
                  const dd = String(today.getDate()).padStart(2, '0');
                  const mm = String(today.getMonth() + 1).padStart(2, '0');
                  const yyyy = today.getFullYear();
                  statusForm.setFieldValue('comment', `Согласовано от ${dd}.${mm}.${yyyy}. `);
                }
              }}
            />
          </Form.Item>
          <Form.Item name="comment" label="Комментарий" rules={[{ required: true }]}>
            <Input.TextArea rows={3} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
