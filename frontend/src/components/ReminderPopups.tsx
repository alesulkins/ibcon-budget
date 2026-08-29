import { useEffect } from 'react';
import { notification } from 'antd';
import { BellOutlined } from '@ant-design/icons';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import dayjs from 'dayjs';
import { remindersApi } from '../api';
import { useUISettings } from '../store/uiSettings';

/**
 * Как часто спрашиваем сервер о наступивших напоминаниях.
 *
 * Минуты достаточно: напоминание на 14:00 всплывёт максимум в 14:01, а
 * опрос раз в несколько секунд ради этого держал бы соединение впустую.
 */
const POLL_MS = 60_000;

/**
 * Всплывающие напоминания. Живут в каркасе приложения, а не на странице
 * личного кабинета: напоминание должно догнать человека там, где он
 * работает, а не ждать, пока он зайдёт в свой профиль.
 *
 * Показ подтверждается серверу сразу: иначе то же напоминание всплывало
 * бы при каждом опросе. Письмо отправляется отдельно и от показа не
 * зависит — почту можно выключить.
 */
export default function ReminderPopups() {
  const qc = useQueryClient();
  const { noticeColor } = useUISettings();

  const { data: due } = useQuery({
    queryKey: ['reminders-due'],
    queryFn: remindersApi.due,
    refetchInterval: POLL_MS,
    // Уведомления не должны сыпаться пачкой после возвращения на
    // вкладку — сервер и так отдаёт только непоказанные.
    refetchOnWindowFocus: true,
  });

  useEffect(() => {
    if (!due || due.length === 0) return;

    for (const r of due) {
      notification.open({
        key: `reminder-${r.id}`,
        message: 'Напоминание',
        description: (
          <div>
            <div style={{ marginBottom: 4 }}>{r.text}</div>
            <div style={{ fontSize: 12, opacity: 0.65 }}>
              {dayjs(r.remind_at).format('DD.MM.YYYY HH:mm')}
            </div>
          </div>
        ),
        icon: <BellOutlined style={{ color: noticeColor }} />,
        // Не закрываем сами: напоминание можно пропустить, отойдя от
        // экрана, — пусть висит, пока его не закроют.
        duration: 0,
        placement: 'bottomRight',
      });
    }

    // Показ подтверждаем сразу после того, как уведомления созданы:
    // повторный опрос не должен принести их снова.
    remindersApi.markShown(due.map(r => r.id))
      .then(() => {
        qc.invalidateQueries({ queryKey: ['reminders-due'] });
        qc.invalidateQueries({ queryKey: ['reminders'] });
      })
      .catch(() => {
        // Подтверждение не прошло — напоминание всплывёт ещё раз при
        // следующем опросе. Это лучше, чем потерять его молча.
      });
  }, [due, qc, noticeColor]);

  return null;
}
