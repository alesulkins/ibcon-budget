import { useState } from 'react';
import {
  Alert, Button, Divider, Form, Input, InputNumber, Modal,
  Space, Table, Tooltip, Typography,
} from 'antd';
import { useMutation } from '@tanstack/react-query';
import { marketApi } from '../../../api';
import { extractError } from '../../../api/client';
import { fmtNum } from '../../../utils/fmt';
import RentWhyPanel from './RentWhyPanel';
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
  const [form] = Form.useForm<RentMarketQuery>();
  const [result, setResult] = useState<RentMarketEstimate | null>(null);

  const ask = useMutation({
    mutationFn: (q: RentMarketQuery) => marketApi.rentEstimate(q),
    onSuccess: setResult,
  });

  function submit(values: RentMarketQuery) {
    setResult(null);
    ask.mutate({
      city: values.city,
      district: values.district || undefined,
      rooms: values.rooms || undefined,
      area: values.area || undefined,
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
        style={{ rowGap: 12, marginBottom: 12, flexWrap: 'wrap' }}
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
        {/* Площадь и кнопка прижаты к правому краю: город с районом
            задают, что искать, а эти два — уточнение и само действие,
            и глазу проще, когда они стоят отдельной группой. */}
        <Form.Item name="area" label="Площадь, м²" style={{ marginLeft: 'auto' }}>
          <InputNumber min={10} max={400} style={{ width: 100 }} />
        </Form.Item>
        <Form.Item style={{ marginRight: 0 }}>
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
        <Stat
          label="В бюджет"
          value={`${fmtNum(est.recommended)} ₽/мес`}
          hint="медиана без верхних 5 % рынка"
          strong
        />
        <Stat
          label="Верх рынка (95-й перцентиль)"
          value={`${fmtNum(est.p95)} ₽/мес`}
          hint="дороже — только 5 % предложений"
        />
        <Stat
          label="Объявлений в расчёте"
          value={String(est.sample)}
          hint={est.matched || 'только помесячная аренда'}
        />
      </div>

      <Histogram est={est} />


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
              { title: 'Площадка', dataIndex: 'source', align: 'center' },
              {
                title: 'Цена, ₽/мес',
                dataIndex: 'price_month',
                align: 'center',
                render: (v: number) => fmtNum(v),
              },
              { title: 'Комнат', dataIndex: 'rooms', align: 'center' },
              {
                title: 'Площадь',
                dataIndex: 'area',
                align: 'center',
                render: (v: number) => `${fmtNum(v)} м²`,
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

      {/* Пояснения — в самом низу: их читают после цифр, а не вместо. */}
      <RentWhyPanel est={est} />
    </div>
  );
}

/**
 * Распределение цен: сколько объявлений в каждом диапазоне.
 *
 * График, а не одни числа: по нему видно, из чего сложилась цена, —
 * плотный ли рынок вокруг медианы или предложения разбросаны. Рисуем
 * своей разметкой, без библиотеки графиков: столбики и две отметки.
 */
function Histogram({ est }: { est: RentMarketEstimate }) {
  const bins = est.histogram ?? [];
  if (bins.length === 0) return null;

  const max = Math.max(...bins.map(b => b.count));
  const lo = bins[0].from;
  const hi = bins[bins.length - 1].to;
  // Доля ширины графика, на которой стоит значение. Столбики идут
  // вплотную, без промежутков, — иначе отметка съезжала бы относительно
  // своего столбика на суммарную ширину зазоров.
  const at = (v: number) => (hi > lo
    ? Math.min(Math.max(((v - lo) / (hi - lo)) * 100, 0), 100)
    : 0);

  const marks = [
    { v: est.recommended, label: 'в бюджет', strong: true },
    { v: est.p95, label: 'верх рынка', strong: false },
  ];

  return (
    <div style={{ marginBottom: 12 }}>
      <div style={{ fontSize: 12, color: 'var(--ibcon-muted)', marginBottom: 4 }}>
        Распределение цен, ₽/мес — {est.sample} объявлений
        {est.matched ? ` (${est.matched})` : ''}
      </div>
      <div style={{ position: 'relative' }}>
        <div style={{ display: 'flex', alignItems: 'flex-end', height: 90 }}>
          {bins.map((b, i) => (
            <Tooltip
              key={i}
              title={`${fmtNum(b.from)} – ${fmtNum(b.to)} ₽/мес · объявлений: ${b.count}`}
            >
              <div style={{
                flex: 1,
                // Пустой диапазон тоже занимает место: провал в середине
                // распределения — это тоже про рынок.
                height: `${max > 0 ? Math.max((b.count / max) * 100, 2) : 2}%`,
                background: 'var(--ibcon-brand)',
                opacity: b.count === 0 ? 0.15 : 0.75,
                // Разделитель внутри столбика, а не зазором между ними:
                // ширина столбиков должна ровно покрывать шкалу.
                boxShadow: 'inset -1px 0 0 var(--ibcon-white)',
              }} />
            </Tooltip>
          ))}
        </div>
        {marks.map(m => (
          <div
            key={m.label}
            style={{
              position: 'absolute',
              left: `${at(m.v)}%`,
              top: 0,
              bottom: 0,
              borderLeft: '1px dashed var(--ibcon-text)',
              opacity: m.strong ? 0.7 : 0.4,
            }}
          />
        ))}
      </div>

      {/* Шкала: края диапазона по бокам, отметки — на своих местах под
          линиями. Подпись под линией, а не в стороне: только так видно,
          что отметка стоит там, где ей положено. */}
      <div style={{
        position: 'relative', height: 30, marginTop: 2,
        fontSize: 11, color: 'var(--ibcon-muted)',
      }}>
        <span style={{ position: 'absolute', left: 0, top: 0 }}>{fmtNum(lo)}</span>
        <span style={{ position: 'absolute', right: 0, top: 0 }}>{fmtNum(hi)}</span>
        {marks.map(m => {
          const x = at(m.v);
          // У краёв шкалы подпись прижимается к краю, а не центрируется
          // по линии: у правого края она иначе уезжает за пределы окна.
          const shift = x > 85 ? '-100%' : x < 15 ? '0' : '-50%';
          return (
            <span
              key={m.label}
              style={{
                position: 'absolute',
                left: `${x}%`,
                top: 14,
                transform: `translateX(${shift})`,
                whiteSpace: 'nowrap',
                color: m.strong ? 'var(--ibcon-text)' : undefined,
              }}
            >
              {m.label} {fmtNum(m.v)}
            </span>
          );
        })}
      </div>
    </div>
  );
}

/** Один показатель: подпись сверху, значение снизу. */
function Stat({ label, value, hint, strong }: {
  label: string; value: string; hint?: string; strong?: boolean;
}) {
  return (
    // По центру колонки: подписи и числа разной длины, и при выключке
    // влево значения стояли лесенкой — глазу не за что зацепиться.
    <div style={{ textAlign: 'center' }}>
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
        { title: 'Площадка', dataIndex: 'source', align: 'center' },
        { title: 'Объявлений', dataIndex: 'count', align: 'center' },
        {
          title: 'Медиана, ₽/мес',
          dataIndex: 'median',
          align: 'center',
          render: (v: number) => (v ? fmtNum(v) : '—'),
        },
        {
          title: 'Ответ',
          dataIndex: 'error',
          align: 'center',
          render: (v: string) => (v
            ? <Text type="warning" style={{ fontSize: 12 }}>{v}</Text>
            : <Text type="success" style={{ fontSize: 12 }}>данные получены</Text>),
        },
      ]}
    />
  );
}
