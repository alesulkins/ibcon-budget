import React, { useState, useEffect } from 'react';
import {
  Card, Button, Space, Typography, Spin, message,
  Modal, Form, Input, Select, Row, Col, Alert,
} from 'antd';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useParams } from 'react-router-dom';
import { budgetsApi, projectsApi } from '../../api';
import { BUDGET_STATUS_LABELS, BUDGET_STATUS_COLORS } from '../../types';
import { hasRole } from '../../store/auth';
import { resetSaveState } from '../../store/autosave';
import { fmtMoney, fmtDate } from '../../utils/fmt';
import Profitability from '../../components/Profitability';
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
import OfficeInput from './inputs/OfficeInput';
import CostLinesInput from './inputs/CostLinesInput';
import PurchasesInput from './inputs/PurchasesInput';
import GphEmployeesInput from './inputs/GphEmployeesInput';
import OverheadInput from './inputs/OverheadInput';
import BudgetParamsInput from './inputs/BudgetParamsInput';
import CalcResults from './CalcResults';
import StatusTag from '../../components/StatusTag';

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
  { key: 'control_equipment', title: 'СК оборудование',    desc: '4.7 – Приборы стройконтроля' },
  { key: 'software',     title: 'ПО',                      desc: '4.8 – Приобретение ПО' },
  { key: 'subcontract_ext', title: 'ГПХ внешний', desc: '4.9 – Субподряд (ГПХ внешний)' },
  { key: 'subcontract_emp', title: 'ГПХ сотрудников',  desc: '4.10 – Субподряд (ГПХ сотрудников)' },
  { key: 'subcontract_gen', title: 'Субподряд',            desc: '4.11' },
  { key: 'corporate_events', title: 'Корпоративы',         desc: '4.12 – Корпоративные мероприятия' },
  { key: 'overhead',     title: 'Прочие расходы',          desc: 'Строки 178-211 (накладные)' },
  { key: 'params',       title: 'Параметры',               desc: 'Непредвиденные, АУП, БГ, маржа' },
  { key: 'results',      title: 'Результаты',              desc: 'Итоги расчёта бюджета' },
];

export default function BudgetVersionPage() {
  const { vid } = useParams<{ vid: string }>();
  const versionId = Number(vid);
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
        // Аренда и уборка офиса; суммы считает бэкенд (calcOffice, лист 4.5).
        // Исполнитель нужен: аренда делится на 0.87, кроме Киргизии.
        return (
          <OfficeInput
            versionId={versionId}
            duration={duration}
            startDate={project!.start_date}
            executor={project!.executor_name}
            readonly={isReadonly}
          />
        );
      case 'control_equipment':
        return (
          <PurchasesInput
            versionId={versionId}
            type="equipment_items"
            title="Приборы строительного контроля"
            duration={duration}
            startDate={project!.start_date}
            readonly={isReadonly}
            nameLabel="Название прибора"
            namePlaceholder="например, Нивелир оптический"
            monthColLabel="Месяц покупки"
            priceLabel="Цена за ед., ₽"
            countLabel="Кол-во"
            addLabel="Добавить прибор"
            hint={'Одна строка — один прибор. Покупка учитывается целиком в '
              + 'месяц приобретения; доступны все месяцы проекта, включая '
              + 'последние два. Сумма уходит одной строкой бюджета «Приборы '
              + 'строительного контроля».'}
          />
        );
      case 'software':
        return (
          <CostLinesInput
            versionId={versionId}
            type="software_items"
            title="ПО и лицензии"
            duration={duration}
            startDate={project!.start_date}
            readonly={isReadonly}
            nameLabel="Наименование ПО"
            namePlaceholder="например, AutoCAD, годовая лицензия"
            addLabel="Добавить ПО"
            hint={'Одна строка — одна позиция ПО или лицензия. Стоимость '
              + 'указывается отдельно по каждому месяцу: пусто или 0 — в этом '
              + 'месяце позиция не оплачивается. Сумма всех позиций уходит '
              + 'одной строкой бюджета «Приобретение ПО».'}
          />
        );
      case 'subcontract_ext':
        return (
          <CostLinesInput
            versionId={versionId}
            type="subcontract_ext_items"
            title="ГПХ внешний"
            duration={duration}
            startDate={project!.start_date}
            readonly={isReadonly}
            nameLabel="Контрагент / услуга"
            namePlaceholder="например, ООО «Геодезия», вынос осей"
            addLabel="Добавить контрагента"
            hint={'Одна строка — один контрагент или услуга. Стоимость '
              + 'указывается отдельно по каждому месяцу: пусто или 0 — в этом '
              + 'месяце оплаты нет. Сумма всех позиций уходит одной строкой '
              + 'бюджета «ГПХ внешний».'}
          />
        );
      case 'subcontract_emp':
        return (
          <GphEmployeesInput
            versionId={versionId}
            duration={duration}
            startDate={project!.start_date}
            readonly={isReadonly}
          />
        );
      case 'subcontract_gen':
        return (
          <CostLinesInput
            versionId={versionId}
            type="subcontract_gen_items"
            title="Субподрядные работы"
            duration={duration}
            startDate={project!.start_date}
            readonly={isReadonly}
            nameLabel="Наименование работ"
            namePlaceholder="например, Монтаж металлоконструкций"
            addLabel="Добавить работы"
            hint={'Одна строка — один вид субподрядных работ. Стоимость '
              + 'указывается отдельно по каждому месяцу: пусто или 0 — в этом '
              + 'месяце оплаты нет. Сумма всех позиций уходит одной строкой '
              + 'бюджета «Субподрядные работы».'}
          />
        );
      case 'corporate_events':
        return (
          <PurchasesInput
            versionId={versionId}
            type="corporate_events_items"
            title="Корпоративные мероприятия"
            duration={duration}
            startDate={project!.start_date}
            readonly={isReadonly}
            nameLabel={null}
            monthColLabel="Месяц проведения"
            priceLabel="Стоимость за чел., ₽"
            countLabel="Кол-во участников"
            addLabel="Добавить корпоратив"
            hint={'Одна строка — один корпоратив. Итог строки — количество '
              + 'участников × стоимость за человека, начисляется в месяц '
              + 'проведения. Корпоративов может не быть совсем: пустая '
              + 'таблица даёт нулевой расход.'}
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
            startDate={project!.start_date}
            // Признак «версию уже считали»: расчёт кэширует эти два поля
            // в budget_versions, из них же берёт цифры реестр.
            calculated={
              version!.cost_no_vat != null || version!.profitability != null
            }
          />
        );
      default:
        return null;
    }
  }

  return (
    // ibcon-blocks — обводка блоков линией: на одном экране их до десятка
    // подряд, и без рамки не видно, где кончается один и начинается
    // следующий. Правило в index.css.
    <div className="ibcon-blocks">
      {/* Возврат к проекту — по хлебным крошкам в шапке. */}
      <Card
        title={
          <Space>
            <Title level={4} style={{ margin: 0 }}>
              {project.name}
              {version.version_label ? ` — ${version.version_label}` : ''}
            </Title>
            <StatusTag color={BUDGET_STATUS_COLORS[version.status]}>
              {BUDGET_STATUS_LABELS[version.status]}
            </StatusTag>
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
              <Profitability value={version.profitability} />
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
        <div style={{ padding: '16px 24px', borderBottom: '1px solid var(--ibcon-line)' }}>
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
          borderTop: '1px solid var(--ibcon-line)',
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
