import React, { useEffect } from 'react';
import { Card, Form, InputNumber, Select, Button, message, Typography, Row, Col, Divider } from 'antd';
import { SaveOutlined } from '@ant-design/icons';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { budgetsApi } from '../../../api';
import type { InputBudgetParams } from '../../../types';
import { extractError } from '../../../api/client';

const { Text } = Typography;

interface Props {
  versionId: number;
  readonly?: boolean;
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
  manual_revenue: 0,
  contract_value: 0,
};

export default function BudgetParamsInput({ versionId, readonly }: Props) {
  const [form] = Form.useForm();
  const qc = useQueryClient();

  const { data: saved } = useQuery({
    queryKey: ['budget-input', versionId, 'budget_params'],
    queryFn: () => budgetsApi.getInput<InputBudgetParams>(versionId, 'budget_params'),
  });

  useEffect(() => {
    if (saved && Object.keys(saved).length > 0) {
      form.setFieldsValue(saved);
    } else {
      form.setFieldsValue(DEFAULTS);
    }
  }, [saved, form]);

  const saveMutation = useMutation({
    mutationFn: (vals: InputBudgetParams) => budgetsApi.saveInput(versionId, 'budget_params', vals),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['budget-input', versionId, 'budget_params'] });
      message.success('Параметры бюджета сохранены');
    },
    onError: (e) => message.error(extractError(e)),
  });

  const numFmt = (v: number | undefined) => v?.toLocaleString('ru-RU') ?? '';

  return (
    <Form form={form} layout="vertical" onFinish={saveMutation.mutate} disabled={readonly}>
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

      <Card title="Банковские гарантии" size="small" style={{ marginBottom: 16 }}>
        <Text type="secondary">
          Суммы рассчитываются от стоимости договора (G251). Если стоимость договора не задана — используется расчётная выручка.
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

      <Card title="Выручка и рентабельность" size="small" style={{ marginBottom: 16 }}>
        <Row gutter={24}>
          <Col span={8}>
            <Form.Item
              name="target_rent_pct"
              label="Целевая рентабельность без НП, %"
              tooltip={
                'Коэффициент наценки на расходы система считает сама из целевой ' +
                'рентабельности и ставки налога исполнителя. Работает, только ' +
                'когда стоимость договора (ТКП) не задана.'
              }
            >
              <InputNumber min={0} max={100} style={{ width: '100%' }} addonAfter="%" />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item
              name="manual_revenue"
              label="Ручная стоимость работ, ₽ (если задана, наценка не применяется)"
            >
              <InputNumber min={0} style={{ width: '100%' }} formatter={numFmt} />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item
              name="contract_value"
              label="Стоимость договора (ТКП), ₽"
              tooltip={
                'От ТКП считаются банковские гарантии и налог киргизского ' +
                'спецрежима. Если ТКП не задан — БГ нулевые.'
              }
            >
              <InputNumber min={0} style={{ width: '100%' }} formatter={numFmt} />
            </Form.Item>
          </Col>
        </Row>
      </Card>

      {!readonly && (
        <Button
          type="primary"
          htmlType="submit"
          icon={<SaveOutlined />}
          loading={saveMutation.isPending}
          style={{ background: '#1a3a6b' }}
        >
          Сохранить параметры
        </Button>
      )}
    </Form>
  );
}
