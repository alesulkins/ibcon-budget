import { useCallback, useEffect, useState } from 'react';
import { Card, Form, InputNumber, Select, Typography, Row, Col, Divider } from 'antd';
import { useQuery } from '@tanstack/react-query';
import { budgetsApi } from '../../../api';
import type { InputBudgetParams } from '../../../types';
import { useAutosave } from '../../../hooks/useAutosave';

const { Text } = Typography;

interface Props {
  versionId: number;
  /** Название исполнителя — от него зависит, задаётся ТКП с НДС или без. */
  executor?: string;
  readonly?: boolean;
}

/** «Айбикон» вводит ТКП с НДС, два других исполнителя — без (2.Бюджет!F251). */
function tkpIncludesVAT(executor?: string): boolean {
  const e = (executor ?? '').trim().toLowerCase();
  return e === 'айбикон';
}

const DEFAULTS: InputBudgetParams = {
  unpredictables_pct: 7,
  aup_pct: 15,
  other_expense_mode: 'млн',
  other_expense_value: 0,
  bg_execution: { pct: 0, rate_pct: 0, rate_type: '%/год', duration_mos: 0 },
  bg_warranty: { pct: 0, rate_pct: 0, rate_type: '%/год', duration_mos: 0 },
  bg_advance: { pct: 0, rate_pct: 0, rate_type: '%/год', duration_mos: 0 },
  target_rent_pct: 0,
  contract_value: 0,
};

export default function BudgetParamsInput({ versionId, executor, readonly }: Props) {
  const [form] = Form.useForm();
  const [values, setValues] = useState<InputBudgetParams>(DEFAULTS);

  const { data: saved, isSuccess } = useQuery({
    queryKey: ['budget-input', versionId, 'budget_params'],
    queryFn: () => budgetsApi.getInput<InputBudgetParams>(versionId, 'budget_params'),
  });

  const [hydrated, setHydrated] = useState(false);

  useEffect(() => {
    if (!isSuccess) return;
    const init = saved && Object.keys(saved).length > 0
      ? { ...DEFAULTS, ...saved }
      : DEFAULTS;
    form.setFieldsValue(init);
    setValues(init);
    setHydrated(true);
  }, [saved, form, isSuccess]);

  const save = useCallback(
    (vals: InputBudgetParams) => budgetsApi.saveInput(versionId, 'budget_params', vals),
    [versionId],
  );

  useAutosave({ data: values, ready: hydrated, save, enabled: !readonly });

  const numFmt = (v: number | undefined) => v?.toLocaleString('ru-RU') ?? '';

  /**
   * Режим выручки. ТКП задан → ручная выручка, банковские гарантии
   * считаются от него. ТКП пуст → режим наценки: БГ не от чего считать
   * (все нулевые), зато работает целевая рентабельность.
   * См. docs/rules_excel.md §4 и §5.
   */
  const hasTKP = (values.contract_value ?? 0) > 0;

  const withVAT = tkpIncludesVAT(executor);
  const vatLabel = withVAT
    ? 'ТКП / стоимость договора с НДС, ₽'
    : 'ТКП / стоимость договора без НДС, ₽';

  // Показываем, во что превратится ТКП: экономисту важно видеть, что
  // в расчёт уйдёт сумма без НДС, а не введённая.
  const net = withVAT
    ? (values.contract_value ?? 0) / 1.22
    : (values.contract_value ?? 0);
  const netRevenueHint = withVAT
    ? `ТКП задан с НДС. Стоимость работ без НДС = ${net.toLocaleString('ru-RU', { maximumFractionDigits: 2 })} ₽ (ТКП / 1.22) — именно она идёт в расчёт выручки.`
    : 'ТКП задан без НДС — стоимость работ равна ему, деление на НДС не применяется.';

  return (
    <Form
      form={form}
      layout="vertical"
      disabled={readonly}
      // Мержим, а не заменяем: секция БГ скрывается при пустом ТКП, и её
      // значения не должны потеряться из отправляемых данных.
      onValuesChange={(_, all) => setValues(prev => ({ ...prev, ...(all as InputBudgetParams) }))}
    >
      <Card title="Накладные коэффициенты" size="small" style={{ marginBottom: 16 }}>
        <Row gutter={24}>
          <Col span={8}>
            <Form.Item name="unpredictables_pct" label="Непредвиденные, %">
              <InputNumber min={0} max={100} style={{ width: '100%' }} addonAfter="%" />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item name="aup_pct" label="АУП, %">
              <InputNumber min={0} max={100} style={{ width: '100%' }} addonAfter="%" />
            </Form.Item>
          </Col>
        </Row>
      </Card>

      <Card title="Прочие расходы (строка 218)" size="small" style={{ marginBottom: 16 }}>
        <Row gutter={16} align="bottom">
          <Col span={8}>
            <Form.Item name="other_expense_mode" label="Режим">
              <Select options={[{ value: 'млн', label: 'Сумма в млн руб' }, { value: '%', label: '% от стоимости договора' }]} />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item name="other_expense_value" label="Значение">
              <InputNumber style={{ width: '100%' }} min={0} />
            </Form.Item>
          </Col>
        </Row>
      </Card>

      {hasTKP && (
      <Card title="Банковские гарантии" size="small" style={{ marginBottom: 16 }}>
        <Text type="secondary">
          Суммы рассчитываются от стоимости договора (ТКП).
        </Text>
        {[
          { prefix: 'bg_execution', label: 'БГ на исполнение обязательств' },
          { prefix: 'bg_warranty',  label: 'БГ на гарантийный период' },
          { prefix: 'bg_advance',   label: 'БГ на аванс' },
        ].map(({ prefix, label }) => (
          <div key={prefix}>
            <Divider style={{ fontSize: 13, margin: '12px 0 8px' }}>{label}</Divider>
            <Row gutter={16}>
              <Col span={6}>
                <Form.Item name={[prefix, 'pct']} label="% от стоимости договора">
                  <InputNumber min={0} max={100} style={{ width: '100%' }} addonAfter="%" />
                </Form.Item>
              </Col>
              <Col span={6}>
                <Form.Item name={[prefix, 'rate_pct']} label="Ставка, %">
                  <InputNumber min={0} style={{ width: '100%' }} addonAfter="%" />
                </Form.Item>
              </Col>
              <Col span={6}>
                <Form.Item name={[prefix, 'rate_type']} label="Тип ставки">
                  <Select options={[
                    { value: '%/год', label: '% в год' },
                    { value: '%/весь срок', label: '% за весь срок' },
                  ]} />
                </Form.Item>
              </Col>
              <Col span={6}>
                <Form.Item name={[prefix, 'duration_mos']} label="Срок, мес.">
                  <InputNumber min={0} style={{ width: '100%' }} />
                </Form.Item>
              </Col>
            </Row>
          </div>
        ))}
      </Card>
      )}

      <Card title="Выручка и рентабельность" size="small" style={{ marginBottom: 16 }}>
        <Row gutter={24}>
          <Col span={8}>
            <Form.Item
              name="contract_value"
              label={vatLabel}
              tooltip={
                'Единственное поле стоимости договора. Стоимость работ без НДС ' +
                'система выводит из него сама (2.Бюджет!G252), отдельно её ' +
                'вводить не нужно. От ТКП также считаются банковские гарантии ' +
                'и налог киргизского спецрежима.'
              }
            >
              <InputNumber min={0} style={{ width: '100%' }} formatter={numFmt} />
            </Form.Item>
          </Col>

          {/* Наценка работает только без ТКП: режимы взаимоисключающие. */}
          {!hasTKP && (
            <Col span={8}>
              <Form.Item
                name="target_rent_pct"
                label="Целевая рентабельность без НП, %"
                tooltip={
                  'Коэффициент наценки на расходы система считает сама из целевой ' +
                  'рентабельности и ставки налога исполнителя. Сумма целевой ' +
                  'рентабельности и ставки налога должна быть меньше 100 %.'
                }
              >
                <InputNumber min={0} max={100} style={{ width: '100%' }} addonAfter="%" />
              </Form.Item>
            </Col>
          )}
        </Row>

        <Text type="secondary" style={{ fontSize: 12 }}>
          {hasTKP
            ? netRevenueHint
            : 'ТКП не задан — режим наценки: выручка считается от расходов через целевую рентабельность, банковские гарантии нулевые.'}
        </Text>
      </Card>
    </Form>
  );
}
