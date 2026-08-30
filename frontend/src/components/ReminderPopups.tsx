import { useEffect } from 'react';
import { notification } from 'antd';
import { BellOutlined } from '@ant-design/icons';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import dayjs from 'dayjs';
import { remindersApi } from '../api';
import { useUISettings } from '../store/uiSettings';

/**
 * Запасной опрос сервера. Основной способ — точный таймер на момент
 * ближайшего срока (см. ниже): опрос раз в минуту давал напоминание с
 * задержкой до минуты, а «в 14:00» должно всплывать в 14:00.
 *
 * Опрос остаётся на случай, когда напоминание завели на другом
 * устройстве: о нём эта вкладка иначе не узнает.
 */
const POLL_MS = 5 * 60_000;

/**
 * Запас на расхождение часов. Сервер отбирает наступившие по своему
 * времени: сработай таймер секунда в секунду по часам браузера, запрос
 * мог бы уйти раньше и вернуть пусто — тогда напоминание ждало бы
 * следующего опроса.
 */
const CLOCK_SKEW_MS = 1_500;

/**
 * Всплывающие напоминания. Живут в каркасе приложения, а не на странице
 * личного кабинета: напоминание должно догнать человека там, где он
 * работает, а не ждать, пока он зайдёт в свой профиль.
 *
 * Показ подтверждается серверу сразу: иначе то же напоминание всплывало
 * бы при каждой проверке.
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
