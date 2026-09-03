import { useState } from 'react';
import { Button, InputNumber, Select, Space, Typography, message } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { refsApi } from '../../api';
import type { CitySalary } from '../../types';
import { extractError } from '../../api/client';
import { thousandFormatter, thousandParser } from '../../utils/fmt';
import DeleteRowButton from '../../components/DeleteRowButton';
import { LINE } from '../../theme';

interface Props {
  value: CitySalary[];
  onChange: (next: CitySalary[]) => void;
  disabled?: boolean;
}

// Оклады должности по городам. В разных городах за одну и ту же работу
// платят по-разному, и в мастер подставляется ставка города проекта.
export default function CitySalaries({ value, onChange, disabled }: Props) {
  const qc = useQueryClient();
  const [newCity, setNewCity] = useState('');
  const [adding, setAdding] = useState(false);

  const { data: cities = [] } = useQuery({
    queryKey: ['cities'],
    queryFn: () => refsApi.cities(),
  });

  // Города, которых ещё нет в списке окладов: один город — одна ставка.
  const used = new Set(value.map(v => v.city_id));
  const free = cities.filter(c => !used.has(c.id));

  async function addCity(name: string) {
    const trimmed = name.trim();
    if (!trimmed) return;
    setAdding(true);
    try {
      const city = await refsApi.createCity(trimmed);
      await qc.invalidateQueries({ queryKey: ['cities'] });
      if (!used.has(city.id)) {
        onChange([...value, { city_id: city.id, city_name: city.name, salary: 0 }]);
      }
      setNewCity('');
    } catch (e) {
      message.error(extractError(e));
    } finally {
      setAdding(false);
    }
  }

  function setSalary(cityId: number, salary: number | null) {
    onChange(value.map(v => (v.city_id === cityId ? { ...v, salary: salary ?? 0 } : v)));
  }

  return (
    <div>
      {value.length > 0 && (
        <div style={{ border: `1px solid ${LINE}`, borderRadius: 8, marginBottom: 8 }}>
          {value.map((cs, i) => (
            <div
              key={cs.city_id}
              style={{
                display: 'flex', alignItems: 'center', gap: 8, padding: '6px 10px',
                borderTop: i === 0 ? 'none' : `1px solid ${LINE}`,
              }}
            >
              <span style={{ flex: 1, minWidth: 0 }}>{cs.city_name}</span>
              <InputNumber
                size="small"
                style={{ width: 160 }}
                min={0}
                value={cs.salary}
                disabled={disabled}
                formatter={thousandFormatter}
                parser={thousandParser}
                onChange={(v) => setSalary(cs.city_id, v)}
              />
              <DeleteRowButton
                title={`Убрать оклад для города «${cs.city_name}»?`}
                onConfirm={() => onChange(value.filter(v => v.city_id !== cs.city_id))}
              />
            </div>
          ))}
        </div>
      )}

      <Space.Compact style={{ width: '100%' }}>
        <Select
          showSearch
          allowClear
          placeholder="Добавить город"
          style={{ flex: 1 }}
          value={null}
          disabled={disabled}
          options={free.map(c => ({ value: c.id, label: c.name }))}
          // Города в списке нет — предлагаем завести его прямо отсюда.
          onSearch={setNewCity}
          onChange={(id: number | null) => {
            const city = cities.find(c => c.id === id);
            if (city) onChange([...value, { city_id: city.id, city_name: city.name, salary: 0 }]);
          }}
          notFoundContent={
            newCity.trim()
              ? (
                <Button
                  type="link"
                  size="small"
                  loading={adding}
                  onClick={() => addCity(newCity)}
                >
                  Создать город «{newCity.trim()}»
                </Button>
              )
              : 'Начните вводить название'
          }
          filterOption={(input, opt) =>
            String(opt?.label ?? '').toLowerCase().includes(input.toLowerCase())}
        />
        <Button
          icon={<PlusOutlined />}
          disabled={disabled || !newCity.trim()}
          loading={adding}
          onClick={() => addCity(newCity)}
          title="Добавить новый город в справочник"
        />
      </Space.Compact>

      <Typography.Text type="secondary" style={{ fontSize: 12, display: 'block', marginTop: 6 }}>
        Для города проекта подставится его ставка. Города без своей ставки
        получают оклад по умолчанию, заданный выше.
      </Typography.Text>
    </div>
  );
}
