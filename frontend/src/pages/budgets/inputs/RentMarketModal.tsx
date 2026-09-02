import { useState } from 'react';
import {
  Alert, Button, Divider, Form, Input, InputNumber, Modal, Select,
  Space, Table, Tag, Typography,
} from 'antd';
import { useMutation } from '@tanstack/react-query';
import { marketApi } from '../../../api';
import { extractError } from '../../../api/client';
import { fmtNum } from '../../../utils/fmt';
import type { RentMarketEstimate, RentMarketQuery } from '../../../types';

const { Text } = Typography;

interface Props {
  open: boolean;
  onClose: () => void;
  /** Город проекта — подставляется в форму, но его можно поменять. */
  defaultCity?: string;
  /** Подставить цену в строку таблицы. Пусто — режим «только посмотреть». */
  onApply?: (rooms: number, price: number) => void;
}

/**
 * Рыночная стоимость аренды квартиры.
 *
 * Платформа опрашивает площадки объявлений и считает по ним оценку.
 * Показываем не только цифру, но и на чём она построена: сколько
 * объявлений собрано, какая площадка что ответила и какая модель
 * считала прогноз. Цифра без этого — гадание, а её ставят в бюджет.
 */
export default function RentMarketModal({ open, onClose, defaultCity, onApply }: Props) {
  const [form] = Form.useForm<RentMarketQuery & { elevator?: string }>();
  const [result, setResult] = useState<RentMarketEstimate | null>(null);

  const ask = useMutation({
    mutationFn: (q: RentMarketQuery) => marketApi.rentEstimate(q),
    onSuccess: setResult,
  });

  function submit(values: RentMarketQuery & { elevator?: string }) {
    setResult(null);
    ask.mutate({
      city: values.city,
      district: values.district || undefined,
      rooms: values.rooms || undefined,
      area: values.area || undefined,
      floor: values.floor || undefined,
      // «Не важно» — не то же самое, что «лифта нет»: пустое значение
      // означает, что признак не задан и объявления по нему не делятся.
      elevator: values.elevator === 'yes' ? true : values.elevator === 'no' ? false : null,
      metro_minutes: values.metro_minutes || undefined,
    });
  }

  const rooms = Form.useWatch('rooms', form) ?? 0;

  return (
    <Modal
      open={open}
      onCancel={onClose}
      title="Рыночная стоимость аренды"
      width={860}
      footer={null}
      destroyOnHidden
    >
      <Form
        form={form}
        layout="inline"
        initialValues={{ city: defaultCity ?? '' }}
        onFinish={submit}
        style={{ rowGap: 12, marginBottom: 12 }}
      >
        <Form.Item
          name="city"
          label="Город"
          rules={[{ required: true, message: 'Город обязателен' }]}
        >
          <Input style={{ width: 180 }} placeholder="Санкт-Петербург" />
        </Form.Item>
        <Form.Item name="district" label="Район">
          <Input style={{ width: 150 }} placeholder="необязательно" />
        </Form.Item>
        <Form.Item name="rooms" label="Комнат">
          <InputNumber min={1} max={6} style={{ width: 90 }} />
        </Form.Item>
        <Form.Item name="area" label="Площадь, м²">
          <InputNumber min={10} max={400} style={{ width: 100 }} />
        </Form.Item>
        <Form.Item name="floor" label="Этаж">
          <InputNumber min={1} max={80} style={{ width: 80 }} />
        </Form.Item>
        <Form.Item name="elevator" label="Лифт">
          <Select
            style={{ width: 130 }}
            placeholder="не важно"
            allowClear
            options={[
              { value: 'yes', label: 'есть' },
              { value: 'no', label: 'нет' },
            ]}
          />
        </Form.Item>
        <Form.Item name="metro_minutes" label="До метро, мин">
          <InputNumber min={1} max={90} style={{ width: 90 }} />
        </Form.Item>
        <Form.Item>
          <Button type="primary" htmlType="submit" loading={ask.isPending}>
            Узнать стоимость
          </Button>
        </Form.Item>
      </Form>

      {ask.isError && (
        <Alert type="error" showIcon message={extractError(ask.error)} />
      )}

      {result && <Result est={result} rooms={rooms} onApply={onApply} onClose={onClose} />}
    </Modal>
  );
}

function Result({
  est, rooms, onApply, onClose,
}: {
  est: RentMarketEstimate;
  rooms: number;
  onApply?: (rooms: number, price: number) => void;
  onClose: () => void;
}) {
  if (est.sample === 0) {
    return (
      <Alert
        type="warning"
        showIcon
        message="Площадки не отдали объявлений"
        description={(
          <div>
            <div style={{ marginBottom: 8 }}>{est.model_reason}</div>
            <SourceTable est={est} />
          </div>
        )}
      />
    );
  }

  // Подставляем то, что предлагается в бюджет, — 95-й перцентиль.
  const applyRooms = rooms >= 1 && rooms <= 3 ? rooms : 0;

  return (
    <div>
      {/* Показатели плиткой, а не описанием: у antd в описании подпись и
          значение делят одну колонку, и на длинных подписях («В бюджет
          (95-й перцентиль)») значение сжималось в столбик по букве. */}
      <div style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fit, minmax(190px, 1fr))',
        gap: 12,
        marginBottom: 12,
      }}>
        <Stat label="В бюджет (95-й перцентиль)" value={`${fmtNum(est.recommended)} ₽/мес`} strong />
        <Stat label="Медиана рынка" value={`${fmtNum(est.p50)} ₽/мес`} />
        <Stat label="75-й перцентиль" value={`${fmtNum(est.p75)} ₽/мес`} />
        <Stat
          label="Прогноз по вашим параметрам"
          value={est.predicted ? `${fmtNum(est.predicted)} ₽/мес` : '—'}
          hint={est.predicted ? `ошибка ±${fmtNum(est.mae)} ₽` : undefined}
        />
        <Stat label="Объявлений в расчёте" value={String(est.sample)} hint={est.model_reason} />
      </div>

      {est.cached && (
        <Text type="secondary" style={{ fontSize: 12 }}>
          Ответ из кэша от {new Date(est.calculated_at).toLocaleString('ru-RU')} —
          площадки опрашиваются не чаще раза в шесть часов.
        </Text>
      )}

      {onApply && (
        <div style={{ marginTop: 12 }}>
          <Space wrap>
            {[1, 2, 3].map(r => (
              <Button
                key={r}
                type={r === applyRooms ? 'primary' : 'default'}
                onClick={() => { onApply(r, Math.round(est.recommended)); onClose(); }}
              >
                Подставить в {r}кк
              </Button>
            ))}
          </Space>
        </div>
      )}

      <Divider style={{ margin: '16px 0 8px' }} />
      <SourceTable est={est} />

      {est.examples.length > 0 && (
        <>
          <Divider style={{ margin: '16px 0 8px' }} />
          <Text type="secondary" style={{ fontSize: 12 }}>Примеры объявлений</Text>
          <Table
            size="small"
            rowKey={(r, i) => `${r.source}-${i}`}
            dataSource={est.examples}
            pagination={false}
            scroll={{ x: 'max-content' }}
            columns={[
              { title: 'Площадка', dataIndex: 'source' },
              {
                title: 'Цена, ₽/мес',
                dataIndex: 'price_month',
                align: 'right',
                render: (v: number, r) => (
                  <Space size={4}>
                    {fmtNum(v)}
                    {r.daily && <Tag color="default">посуточно ×30</Tag>}
                  </Space>
                ),
              },
              { title: 'Комнат', dataIndex: 'rooms', render: (v: number) => v || '—' },
              {
                title: 'Площадь',
                dataIndex: 'area',
                render: (v: number) => (v ? `${fmtNum(v)} м²` : '—'),
              },
              {
                title: '',
                dataIndex: 'url',
                render: (v: string) => (v
                  ? <a href={v} target="_blank" rel="noreferrer">объявление</a>
                  : null),
              },
            ]}
          />
        </>
      )}
    </div>
  );
}

/** Один показатель: подпись сверху, значение снизу. */
function Stat({ label, value, hint, strong }: {
  label: string; value: string; hint?: string; strong?: boolean;
}) {
  return (
    <div>
      <div style={{ fontSize: 12, color: 'var(--ibcon-muted)' }}>{label}</div>
      <div style={{ fontSize: strong ? 20 : 16, fontWeight: strong ? 600 : 500 }}>{value}</div>
      {hint && <div style={{ fontSize: 11, color: 'var(--ibcon-muted)' }}>{hint}</div>}
    </div>
  );
}

/** Что ответила каждая площадка. Недоступная площадка — не ошибка
    запроса: оценка считается по остальным, но человек должен видеть, чего
    в ней нет. */
function SourceTable({ est }: { est: RentMarketEstimate }) {
  return (
    <Table
      size="small"
      rowKey="source"
      dataSource={est.sources}
      pagination={false}
      columns={[
        { title: 'Площадка', dataIndex: 'source' },
        { title: 'Объявлений', dataIndex: 'count', align: 'right' },
        {
          title: 'Медиана, ₽/мес',
          dataIndex: 'median',
          align: 'right',
          render: (v: number) => (v ? fmtNum(v) : '—'),
        },
        {
          title: 'Ответ',
          dataIndex: 'error',
          render: (v: string) => (v
            ? <Text type="warning" style={{ fontSize: 12 }}>{v}</Text>
            : <Text type="success" style={{ fontSize: 12 }}>данные получены</Text>),
        },
      ]}
    />
  );
}
