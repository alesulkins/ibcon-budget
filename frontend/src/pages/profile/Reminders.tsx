import { useState } from 'react';
import {
  Button, Checkbox, DatePicker, Empty, Input, Space, Switch, Tooltip,
  Typography, message,
} from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import dayjs, { type Dayjs } from 'dayjs';
import { remindersApi, profileApi } from '../../api';
import type { Reminder } from '../../types';
import { extractError } from '../../api/client';
import DeleteRowButton from '../../components/DeleteRowButton';
import { LINE, TEXT_SOFT, STATUS } from '../../theme';

interface Props {
  /** Слать ли напоминания письмом — общий тумблер из профиля. */
  emailReminders: boolean;
}

/**
 * Напоминания личного кабинета: заметка со сроком, которая в срок
 * всплывает уведомлением на экране, а при включённой почте ещё и уходит
 * письмом.
 *
 * Тумблер почты один на все напоминания, а не на каждое: это настройка
 * доставки, а не свойство записи.
 */
export default function Reminders({ emailReminders }: Props) {
  const qc = useQueryClient();
  const [text, setText] = useState('');
  const [at, setAt] = useState<Dayjs | null>(null);

  const { data: items = [], isLoading } = useQuery({
    queryKey: ['reminders'],
    queryFn: remindersApi.list,
  });

  const invalidate = () => qc.invalidateQueries({ queryKey: ['reminders'] });

  const createMutation = useMutation({
    mutationFn: () => remindersApi.create(text.trim(), at!.toISOString()),
    onSuccess: () => {
      invalidate();
      setText('');
      setAt(null);
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

  const emailMutation = useMutation({
    mutationFn: (on: boolean) => profileApi.update({ email_reminders: on }),
    onSuccess: (_, on) => {
      qc.invalidateQueries({ queryKey: ['profile'] });
      message.success(on
        ? 'Напоминания будут приходить на почту'
        : 'Напоминания будут только всплывать на экране');
    },
    onError: (e) => message.error(extractError(e)),
  });

  const canAdd = text.trim() !== '' && at !== null;

  return (
    <div>
      <Space align="center" size={8} style={{ marginBottom: 12 }}>
        <Switch
          size="small"
          checked={emailReminders}
          loading={emailMutation.isPending}
          onChange={(on) => emailMutation.mutate(on)}
        />
        <Typography.Text style={{ fontSize: 13 }}>
          Отправлять напоминания на почту
        </Typography.Text>
        <Tooltip title="Выключено — напоминание только всплывает на экране, письмо не отправляется.">
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>?</Typography.Text>
        </Tooltip>
      </Space>

      <Space.Compact style={{ width: '100%', marginBottom: 12 }}>
        <Input
          placeholder="О чём напомнить"
          value={text}
          onChange={(e) => setText(e.target.value)}
          onPressEnter={() => { if (canAdd) createMutation.mutate(); }}
        />
        <DatePicker
          showTime={{ format: 'HH:mm' }}
          format="DD.MM.YYYY HH:mm"
          placeholder="Когда"
          value={at}
          onChange={setAt}
          style={{ width: 200 }}
        />
        <Button
          type="primary"
          icon={<PlusOutlined />}
          disabled={!canAdd}
          loading={createMutation.isPending}
          onClick={() => createMutation.mutate()}
        />
      </Space.Compact>

      {items.length === 0 && !isLoading && (
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description="Напоминаний пока нет"
        />
      )}

      {items.map((r: Reminder) => {
        // Просроченным считается только незакрытое: выполненное дело
        // красным помечать незачем.
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
              <div style={{ fontSize: 12, color: overdue ? STATUS.red : TEXT_SOFT }}>
                {dayjs(r.remind_at).format('DD.MM.YYYY HH:mm')}
                {overdue && ' · срок прошёл'}
                {r.emailed_at && ' · письмо отправлено'}
              </div>
            </div>
            <DeleteRowButton
              title="Удалить напоминание?"
              onConfirm={() => removeMutation.mutate(r.id)}
            />
          </div>
        );
      })}
    </div>
  );
}
