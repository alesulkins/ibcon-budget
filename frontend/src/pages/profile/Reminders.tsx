import { useState } from 'react';
import {
  Button, Checkbox, DatePicker, Input, Modal, Typography, message,
} from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import dayjs, { type Dayjs } from 'dayjs';
import { remindersApi } from '../../api';
import type { Reminder } from '../../types';
import { extractError } from '../../api/client';
import DeleteRowButton from '../../components/DeleteRowButton';
import { LINE, TEXT_SOFT, STATUS } from '../../theme';

/**
 * Напоминания личного кабинета: заметка со сроком, которая в срок
 * всплывает уведомлением на экране.
 *
 * Поля ввода не висят пустыми над списком — вместо них строка-приглашение
 * «Добавить напоминание». Форма открывается по нажатию: пока напоминание
 * не пишут, место занимает сам список, а не заготовка под него.
 */
export default function Reminders() {
  const qc = useQueryClient();
  const [adding, setAdding] = useState(false);
  const [text, setText] = useState('');
  const [at, setAt] = useState<Dayjs | null>(null);

  const { data: items = [] } = useQuery({
    queryKey: ['reminders'],
    queryFn: remindersApi.list,
  });

  const invalidate = () => qc.invalidateQueries({ queryKey: ['reminders'] });

  const closeForm = () => {
    setAdding(false);
    setText('');
    setAt(null);
  };

  const createMutation = useMutation({
    mutationFn: () => remindersApi.create(text.trim(), at!.toISOString()),
    onSuccess: () => {
      invalidate();
      closeForm();
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

  const canSave = text.trim() !== '' && at !== null;

  return (
    <div>
      {/* Приглашение вместо пустой формы: список важнее заготовки. */}
      <Button
        type="dashed"
        block
        icon={<PlusOutlined />}
        style={{ marginBottom: 12 }}
        onClick={() => setAdding(true)}
      >
        Добавить напоминание
      </Button>

      {items.length === 0 && (
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          Напоминание всплывёт на экране в указанное время — на любой
          странице, а не только здесь.
        </Typography.Text>
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
              </div>
            </div>
            <DeleteRowButton
              title="Удалить напоминание?"
              onConfirm={() => removeMutation.mutate(r.id)}
            />
          </div>
        );
      })}

      <Modal
        title="Новое напоминание"
        open={adding}
        onCancel={closeForm}
        onOk={() => createMutation.mutate()}
        okText="Добавить"
        cancelText="Отмена"
        okButtonProps={{ disabled: !canSave }}
        confirmLoading={createMutation.isPending}
        width={420}
      >
        <Input.TextArea
          placeholder="О чём напомнить"
          value={text}
          onChange={(e) => setText(e.target.value)}
          rows={3}
          style={{ marginBottom: 12 }}
          autoFocus
        />
        <DatePicker
          showTime={{ format: 'HH:mm' }}
          format="DD.MM.YYYY HH:mm"
          placeholder="Когда напомнить"
          value={at}
          onChange={setAt}
          style={{ width: '100%' }}
        />
      </Modal>
    </div>
  );
}
