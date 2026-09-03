import { useEffect } from 'react';
import { notification } from 'antd';
import { BellOutlined } from '@ant-design/icons';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import dayjs from 'dayjs';
import { remindersApi } from '../api';
import { useUISettings } from '../store/uiSettings';

// Запасной опрос сервера. Основной способ — точный таймер на момент
// ближайшего срока (см.
const POLL_MS = 5 * 60_000;

// Запас на расхождение часов.
const CLOCK_SKEW_MS = 1_500;

// Всплывающие напоминания.
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

  // Список всех напоминаний нужен, чтобы знать ближайший срок: выборка
  // наступивших о будущем ничего не говорит.
  const { data: all } = useQuery({
    queryKey: ['reminders'],
    queryFn: remindersApi.list,
  });

  /**
   * Точный таймер на ближайший срок. Без него напоминание всплывало бы
   * в следующий заход опроса — с задержкой до минуты.
   */
  useEffect(() => {
    if (!all) return;
    const now = Date.now();
    const next = all
      .filter(r => !r.done && !r.shown_at)
      .map(r => new Date(r.remind_at).getTime())
      .filter(t => t > now)
      .sort((a, b) => a - b)[0];
    if (next === undefined) return;

    // setTimeout ограничен ~24 днями: на более далёкий срок таймер не
    // ставим — до него сработает обычный опрос.
    const delay = next - now + CLOCK_SKEW_MS;
    if (delay > 2_000_000_000) return;

    const id = window.setTimeout(() => {
      qc.invalidateQueries({ queryKey: ['reminders-due'] });
    }, delay);
    return () => clearTimeout(id);
  }, [all, qc]);

  useEffect(() => {
    if (!due || due.length === 0) return;

    for (const r of due) {
      notification.open({
        key: `reminder-${r.id}`,
        // title, а не message: в antd v6 message у уведомления объявлен
        // устаревшим и ругается в консоли.
        title: 'Напоминание',
        description: (
          <div>
            <div style={{ marginBottom: 4 }}>{r.text}</div>
            <div style={{ fontSize: 12, opacity: 0.65 }}>
              {dayjs(r.remind_at).format('DD.MM.YYYY HH:mm')}
            </div>
          </div>
        ),
        // Знак на цветной подложке — цветом текста, а не цветом
        // подложки: иначе он на ней пропадал.
        icon: <BellOutlined style={{ color: 'var(--ibcon-notice-fg)' }} />,
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
