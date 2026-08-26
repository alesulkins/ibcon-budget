import React, { useState, useEffect } from 'react';
import {
  Card, Button, Space, Tag, Typography, Spin, message,
  Modal, Form, Input, Select, Row, Col, Alert,
} from 'antd';
import { ArrowLeftOutlined } from '@ant-design/icons';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useParams, useNavigate } from 'react-router-dom';
import { budgetsApi, projectsApi } from '../../api';
import { BUDGET_STATUS_LABELS, BUDGET_STATUS_COLORS } from '../../types';
import { hasRole } from '../../store/auth';
import { resetSaveState } from '../../store/autosave';
import { fmtMoney, fmtPct, fmtDate } from '../../utils/fmt';
import { extractError } from '../../api/client';
import WizardSteps from '../../components/WizardSteps';
import { useStickyState } from '../../hooks/useStickyState';
import SaveIndicator from '../../components/SaveIndicator';
import { useUnsavedWarning } from '../../hooks/useUnsavedWarning';
import EmployeesInput from './inputs/EmployeesInput';
import BonusesInput from './inputs/BonusesInput';
import RentApartmentsInput from './inputs/RentApartmentsInput';
import TransportInput from './inputs/TransportInput';
import WagonciksInput from './inputs/WagonciksInput';
import SimpleCostInput from './inputs/SimpleCostInput';
import OverheadInput from './inputs/OverheadInput';
import BudgetParamsInput from './inputs/BudgetParamsInput';
import CalcResults from './CalcResults';

const { Title, Text } = Typography;

/** «Согласовано от ДД.ММ.ГГГГ. » — обязательное начало комментария. */
function approvalPrefixText(): string {
  const d = new Date();
  const dd = String(d.getDate()).padStart(2, '0');
  const mm = String(d.getMonth() + 1).padStart(2, '0');
  return `Согласовано от ${dd}.${mm}.${d.getFullYear()}. `;
}

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
  // Открытый шаг мастера переживает переход в справочники и обратно
  const [step, setStep] = useStickyState(`wizard-step:${vid}`, 0);
  const [statusForm] = Form.useForm();
  const [showStatus, setShowStatus] = useState(false);
  // Неудаляемый префикс комментария при согласовании
  const [approvalPrefix, setApprovalPrefix] = useState('');

  const { data: version, isLoading } = useQuery({
    queryKey: ['budget-version', versionId],
    queryFn: () => budgetsApi.getVersion(versionId),
  });

  // Закрытие вкладки с несохранённым — предупреждаем
  useUnsavedWarning();

  // Индикатор автосохранения не должен показывать чужое время
  // при переходе на другую версию бюджета.
  useEffect(() => {
    resetSaveState();
    return () => resetSaveState();
  }, [versionId]);

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
            executor={project!.executor_name}
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
        // Покупка авто, аренда авто и аренда гаража; суммы считает
        // бэкенд (calcTransport, лист 4.3).
        return (
          <TransportInput
            versionId={versionId}
            duration={duration}
            startDate={project!.start_date}
            readonly={isReadonly}
          />
        );
      case 'site_setup':
        // Аренда и покупка вагончиков; сумму считает бэкенд
        // (calcWagonciks, лист 4.4).
        return (
          <WagonciksInput
            versionId={versionId}
            duration={duration}
            startDate={project!.start_date}
            readonly={isReadonly}
          />
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
            startDate={project!.start_date}
            readonly={isReadonly}
          />
        );
      case 'params':
        return (
          <BudgetParamsInput
            versionId={versionId}
            executor={project!.executor_name}
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
          <WizardSteps
            items={isAP ? [WIZARD_STEPS[apOnlyStepIdx]] : WIZARD_STEPS}
            current={isAP ? 0 : step}
            onChange={isAP ? undefined : setStep}
          />
        </div>
        <div style={{ padding: 24 }}>
          {isAP
            ? (
              <OverheadInput
                versionId={versionId}
                duration={project.duration_months}
                startDate={project.start_date}
                readonly
              />
            )
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
                setApprovalPrefix(v === 'approved' ? approvalPrefixText() : '');
                statusForm.setFieldValue(
                  'comment',
                  v === 'approved' ? approvalPrefixText() : '',
                );
              }}
            />
          </Form.Item>
          <Form.Item
            name="comment"
            label="Комментарий"
            rules={[
              { required: true, message: 'Укажите причину изменения статуса' },
              // Префикс «Согласовано от ДД.ММ.ГГГГ» стереть нельзя —
              // он фиксирует дату согласования.
              {
                validator: (_, value: string) =>
                  !approvalPrefix || (value ?? '').startsWith(approvalPrefix)
                    ? Promise.resolve()
                    : Promise.reject(new Error(
                      `Комментарий должен начинаться с «${approvalPrefix.trim()}»`)),
              },
            ]}
            extra={approvalPrefix
              ? 'Дата согласования подставлена автоматически и не удаляется.'
              : undefined}
          >
            <Input.TextArea
              rows={3}
              onChange={(e) => {
                // Не даём стереть префикс правкой изнутри поля
                if (approvalPrefix && !e.target.value.startsWith(approvalPrefix)) {
                  statusForm.setFieldValue('comment', approvalPrefix);
                }
              }}
            />
          </Form.Item>
        </Form>
      </Modal>

      {/* «сохранено N сек назад» — снизу по центру */}
      <SaveIndicator />
    </div>
  );
}
