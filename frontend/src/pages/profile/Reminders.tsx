import { useRef, useState } from 'react';
import {
  Button, Checkbox, DatePicker, Input, Popover, Space, Tooltip, Typography, message,
} from 'antd';
import { ClockCircleOutlined, DeleteOutlined } from '@ant-design/icons';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import dayjs, { type Dayjs } from 'dayjs';
import { remindersApi } from '../../api';
import type { Reminder } from '../../types';
import { extractError } from '../../api/client';
import { LINE, TEXT_SOFT } from '../../theme';

const { Text } = Typography;

/** Разбор времени, введённого руками: «9:5», «09:05», «0905». */
function parseTime(raw: string): { h: number; m: number } | null {
  const digits = raw.replace(/\D/g, '');
  let h: number;
  let m: number;
  if (raw.includes(':')) {
    const [a, b] = raw.split(':');
    h = Number(a);
    m = Number(b);
  } else if (digits.length === 4) {
    h = Number(digits.slice(0, 2));
    m = Number(digits.slice(2));
  } else {
    return null;
  }
  if (!Number.isFinite(h) || !Number.isFinite(m)) return null;
  if (h < 0 || h > 23 || m < 0 || m > 59) return null;
  return { h, m };
}

/**
 * Выбор срока: дата календарём или руками, время — только руками.
 * Раскрывающийся список часов и минут для «через двадцать минут»
 * медленнее, чем набрать четыре цифры.
 */
function WhenPicker({ date, time, onDate, onTime, open, onOpenChange, onSubmit, children }: {
  date: Dayjs | null;
  time: string;
  onDate: (d: Dayjs | null) => void;
  onTime: (t: string) => void;
  open: boolean;
  onOpenChange: (v: boolean) => void;
  /** Enter в сроке — то же, что «Готово». */
  onSubmit: () => void;
  children: React.ReactNode;
}) {
  return (
    <Popover
      trigger="click"
      placement="bottomRight"
      title="Когда напомнить"
      open={open}
      onOpenChange={onOpenChange}
      content={(
        <Space
          direction="vertical"
          size={8}
          style={{ width: 220 }}
          // Обработчик один на всю панель: Enter в поле времени и Enter
          // в календаре приходят сюда всплытием, и напоминание не
          // сохраняется дважды.
          onKeyDown={(e) => {
            if (e.key === 'Enter') onSubmit();
          }}
        >
          <DatePicker
            format="DD.MM.YYYY"
            placeholder="Дата"
            value={date}
            onChange={onDate}
            style={{ width: '100%' }}
            // Календарь и ручной ввод разом: antd принимает набранную
            // дату в том же поле.
            allowClear={false}
          />
          <Input
            placeholder="Время, ЧЧ:ММ"
            value={time}
            onChange={(e) => onTime(e.target.value)}
            maxLength={5}
            status={time !== '' && parseTime(time) === null ? 'error' : undefined}
          />
          <Text type="secondary" style={{ fontSize: 12 }}>
            Время вводится вручную: например, 09:30. Enter сохраняет напоминание.
          </Text>
        </Space>
      )}
    >
      {children}
    </Popover>
  );
}

/**
 * Напоминания личного кабинета: заметка со сроком, которая в срок
 * всплывает уведомлением на экране.
 *
 * Первая строка списка — не кнопка, а само напоминание, только бледное:
 * заготовка стоит там же, где появится готовая запись, поэтому список не
 * прыгает, а «добавить» не выглядит отдельным действием.
 */
export default function Reminders() {
  const qc = useQueryClient();
  const [text, setText] = useState('');
  const [date, setDate] = useState<Dayjs | null>(null);
  const [time, setTime] = useState('');
  const [pickerOpen, setPickerOpen] = useState(false);

  const { data: items = [] } = useQuery({
    queryKey: ['reminders'],
    queryFn: remindersApi.list,
  });

  const invalidate = () => qc.invalidateQueries({ queryKey: ['reminders'] });

  const createMutation = useMutation({
    mutationFn: () => {
      const t = parseTime(time)!;
      const when = (date ?? dayjs()).hour(t.h).minute(t.m).second(0).millisecond(0);
      return remindersApi.create(text.trim(), when.toISOString());
    },
    onSuccess: () => {
      invalidate();
      setText('');
      setDate(null);
      setTime('');
      message.success('Напоминание добавлено');
    },
    onError: (e) => message.error(extractError(e)),
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, done }: { id: number; done: boolean }) =>
      remindersApi.update(id, { done }),
    onSuccess: invalidate,
    onError: (e) => message.error(extractError(e)),
  });

  const removeMutation = useMutation({
    mutationFn: (id: number) => remindersApi.remove(id),
    onSuccess: () => { invalidate(); message.success('Напоминание удалено'); },
    onError: (e) => message.error(extractError(e)),
  });

  const parsed = parseTime(time);
  const canSave = text.trim() !== '' && parsed !== null;

  /**
   * Enter в панели срока. Значение читается на следующем такте: Enter в
   * календаре сначала подтверждает набранную дату, и в этот момент
   * состояние ещё прежнее — сохранили бы вчерашний срок.
   */
  const draft = useRef({ text, time });
  draft.current = { text, time };
  function submitFromPicker() {
    setTimeout(() => {
      const { text: t, time: tm } = draft.current;
      setPickerOpen(false);
      if (t.trim() !== '' && parseTime(tm) !== null) createMutation.mutate();
    }, 0);
  }
  // Дата не выбрана — считаем сегодняшнюю: «напомнить в 18:00» обычно
  // про сегодня, и лишний клик по календарю тут не нужен.
  const draftWhen = parsed
    ? (date ?? dayjs()).hour(parsed.h).minute(parsed.m)
    : date;

  const row = (extra: React.ReactNode, body: React.ReactNode, faded = false) => (
    <div style={{
      display: 'flex', alignItems: 'flex-start', gap: 8,
      padding: '8px 0', borderTop: `1px solid ${LINE}`,
      opacity: faded ? 0.65 : 1,
    }}>
      {body}
      {extra}
    </div>
  );

  return (
    <div>
      {/* Заготовка в виде напоминания: то же место, тот же строй. */}
      {row(
        <Space size={2}>
          <WhenPicker
            date={date}
            time={time}
            onDate={setDate}
            onTime={setTime}
            open={pickerOpen}
            onOpenChange={setPickerOpen}
            onSubmit={submitFromPicker}
          >
            <Tooltip title="Выбрать дату и время">
              <Button
                size="small"
                type="text"
                icon={<ClockCircleOutlined />}
                style={{ color: draftWhen ? undefined : TEXT_SOFT }}
              />
            </Tooltip>
          </WhenPicker>
          {canSave && (
            <Button
              size="small"
              type="primary"
              loading={createMutation.isPending}
              onClick={() => createMutation.mutate()}
            >
              Готово
            </Button>
          )}
        </Space>,
        <>
          <Checkbox disabled style={{ marginTop: 2 }} />
          <div style={{ flex: 1, minWidth: 0 }}>
            <Input
              variant="borderless"
              placeholder="Напомнить о…"
              value={text}
              onChange={(e) => setText(e.target.value)}
              onPressEnter={() => { if (canSave) createMutation.mutate(); }}
              style={{ padding: 0 }}
            />
            <div style={{ fontSize: 12, color: TEXT_SOFT, paddingLeft: 2 }}>
              {draftWhen
                ? draftWhen.format('DD.MM.YYYY HH:mm')
                : 'дата и время — по часам справа'}
            </div>
          </div>
        </>,
        true,
      )}

      {items.map((r: Reminder) => {
        // Просроченным считается только незакрытое: выполненное дело
        // помечать незачем.
        const overdue = !r.done && dayjs(r.remind_at).isBefore(dayjs());
        return (
          <div
            key={r.id}
            style={{
              display: 'flex', alignItems: 'flex-start', gap: 8,
              padding: '8px 0', borderTop: `1px solid ${LINE}`,
            }}
          >
            <Checkbox
              checked={r.done}
              onChange={(e) => updateMutation.mutate({ id: r.id, done: e.target.checked })}
              style={{ marginTop: 2 }}
            />
            <div style={{ flex: 1, minWidth: 0 }}>
              <div style={{
                textDecoration: r.done ? 'line-through' : undefined,
                color: r.done ? TEXT_SOFT : undefined,
                wordBreak: 'break-word',
              }}>
                {r.text}
              </div>
              {/* Просрочка — не ошибка, а состояние: подсвечиваем тем же
                  приглушённым тоном, что и остальные подписи, только
                  плотнее. Красный здесь читался как поломка. */}
              <div style={{
                fontSize: 12,
                color: TEXT_SOFT,
                fontWeight: overdue ? 600 : 400,
              }}>
                {dayjs(r.remind_at).format('DD.MM.YYYY HH:mm')}
                {overdue && ' · срок прошёл'}
              </div>
            </div>
            <Button
              size="small"
              type="text"
              icon={<DeleteOutlined />}
              title="Удалить напоминание"
              style={{ color: TEXT_SOFT }}
              onClick={() => removeMutation.mutate(r.id)}
            />
          </div>
        );
      })}
    </div>
  );
}
