import { Input, InputNumber, Select, Button, Typography } from 'antd';
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import type { ItemPurchase } from '../types';
import { thousandFormatter, thousandParser, fmtNum } from '../utils/fmt';
import { monthGridCell, monthGridHeadCell } from './MonthGrid';

const { Text } = Typography;

const DEL_COL = 40;
const COUNT_COL = 70;
const TOTAL_COL = 110;

const table: React.CSSProperties = {
  width: '100%',
  tableLayout: 'fixed',
  borderCollapse: 'collapse',
};

interface Props {
  items: ItemPurchase[];
  onChange: (next: ItemPurchase[]) => void;
  /** Подписи месяцев проекта по порядку. */
  months: string[];
  /**
   * Сколько первых месяцев доступно для выбора. По умолчанию все месяцы
   * проекта; лист 4.4 закрывает последние два.
   */
  allowedMonths?: number;
  readonly?: boolean;
  namePlaceholder: string;
  addLabel: string;
  emptyLabel: string;
}

export function emptyPurchase(): ItemPurchase {
  return { name: '', month: 0, count: 1, price: 0 };
}

/**
 * Таблица разовых покупок: описание, месяц, цена за единицу, количество.
 * Итог строки — цена × количество, начисляется целиком в месяц покупки
 * (4.3!D17 и 4.4!D12 = IF(месяц<=$D$8, стоимость*количество, 0)).
 *
 * Общая для покупки авто (4.3) и покупки вагончиков (4.4): владелец
 * потребовал, чтобы блоки были идентичны.
 */
export default function PurchaseTable({
  items, onChange, months, allowedMonths, readonly,
  namePlaceholder, addLabel, emptyLabel,
}: Props) {
  const allowed = allowedMonths ?? months.length;
  // Месяц выбирается только из доступных — вне диапазона не выбрать в принципе.
  const monthOptions = months.slice(0, allowed).map((label, i) => ({ value: i + 1, label }));

  function patch(idx: number, p: Partial<ItemPurchase>) {
    onChange(items.map((r, i) => (i === idx ? { ...r, ...p } : r)));
  }

  return (
    <div>
      <table style={table}>
        <colgroup>
          <col />
          <col style={{ width: 180 }} />
          <col style={{ width: 160 }} />
          <col style={{ width: COUNT_COL }} />
          <col style={{ width: TOTAL_COL }} />
          {!readonly && <col style={{ width: DEL_COL }} />}
        </colgroup>
        <thead>
          <tr>
            <th style={{ ...monthGridHeadCell, textAlign: 'left', color: '#333', fontWeight: 500 }}>
              Описание
            </th>
            <th style={{ ...monthGridHeadCell, color: '#333', fontWeight: 500 }}>
              Месяц покупки
            </th>
            <th style={{ ...monthGridHeadCell, color: '#333', fontWeight: 500 }}>
              Цена за ед., ₽
            </th>
            <th style={{ ...monthGridHeadCell, color: '#333', fontWeight: 500 }}>
              Кол-во
            </th>
            <th style={{ ...monthGridHeadCell, textAlign: 'right', color: '#333', fontWeight: 500 }}>
              Итого
            </th>
            {!readonly && <th style={monthGridHeadCell} />}
          </tr>
        </thead>
        <tbody>
          {items.length === 0 && (
            <tr>
              <td
                colSpan={5 + (readonly ? 0 : 1)}
                style={{ ...monthGridCell, color: '#999', fontSize: 12, padding: '10px 4px' }}
              >
                {emptyLabel}
              </td>
            </tr>
          )}
          {items.map((p, idx) => (
            <tr key={idx} style={{ borderTop: '1px solid #f0f0f0' }}>
              <td style={{ ...monthGridCell, textAlign: 'left' }}>
                {readonly ? (
                  <Text style={{ fontSize: 12 }}>{p.name || '—'}</Text>
                ) : (
                  <Input
                    size="small"
                    value={p.name}
                    placeholder={namePlaceholder}
                    onChange={e => patch(idx, { name: e.target.value })}
                  />
                )}
              </td>
              <td style={monthGridCell}>
                {readonly ? (
                  <Text style={{ fontSize: 12 }}>
                    {p.month >= 1 ? months[p.month - 1] ?? '—' : '—'}
                  </Text>
                ) : (
                  <Select
                    size="small"
                    style={{ width: '100%' }}
                    options={monthOptions}
                    value={p.month >= 1 && p.month <= allowed ? p.month : undefined}
                    placeholder="выберите месяц"
                    status={p.month >= 1 && p.month <= allowed ? undefined : 'error'}
                    onChange={v => patch(idx, { month: v ?? 0 })}
                  />
                )}
              </td>
              <td style={monthGridCell}>
                {readonly ? (
                  <Text style={{ fontSize: 12 }}>{fmtNum(p.price)}</Text>
                ) : (
                  <InputNumber
                    size="small"
                    style={{ width: '100%' }}
                    min={0}
                    value={p.price || null}
                    onChange={v => patch(idx, { price: v ?? 0 })}
                    formatter={thousandFormatter}
                    parser={thousandParser}
                  />
                )}
              </td>
              <td style={monthGridCell}>
                {readonly ? (
                  <Text style={{ fontSize: 12 }}>{p.count}</Text>
                ) : (
                  <InputNumber
                    size="small"
                    style={{ width: '100%' }}
                    min={0}
                    value={p.count}
                    onChange={v => patch(idx, { count: v ?? 0 })}
                  />
                )}
              </td>
              <td style={{ ...monthGridCell, textAlign: 'right', fontWeight: 500, fontSize: 12 }}>
                {fmtNum(p.price * p.count)}
              </td>
              {!readonly && (
                <td style={monthGridCell}>
                  <Button
                    size="small"
                    type="text"
                    danger
                    icon={<DeleteOutlined />}
                    onClick={() => onChange(items.filter((_, i) => i !== idx))}
                  />
                </td>
              )}
            </tr>
          ))}
        </tbody>
      </table>
      {!readonly && (
        <Button
          size="small"
          type="dashed"
          icon={<PlusOutlined />}
          style={{ marginTop: 8 }}
          onClick={() => onChange([...items, emptyPurchase()])}
        >
          {addLabel}
        </Button>
      )}
    </div>
  );
}
