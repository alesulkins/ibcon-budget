import { useCallback, useEffect, useMemo, useRef } from 'react';
import { markSaving, markSaved, markSaveError } from '../store/autosave';
import { extractError } from '../api/client';

/** Пауза после последнего ввода, после которой данные уходят на сервер. */
const DEBOUNCE_MS = 1_000;

/** Страховочное сохранение — ТЗ требует автосохранение черновика раз в 2 минуты. */
const INTERVAL_MS = 2 * 60 * 1_000;

interface Options<T> {
  /** Текущее состояние формы. */
  data: T;
  /**
   * Данные загружены с сервера И РАЗЛОЖЕНЫ по состоянию формы.
   *
   * Именно второе условие критично: если поднять флаг в тот же рендер,
   * когда пришёл ответ, слепок снимется с ещё пустой формы, и следующий
   * же рендер (уже с данными) будет принят за правку пользователя —
   * каждый заход на шаг слал бы на сервер то, что только что с него
   * прочитали. Поэтому формы поднимают флаг в эффекте гидрации.
   */
  ready: boolean;
  save: (data: T) => Promise<unknown>;
  /** Выключено для readonly-версий (согласованных и архивных). */
  enabled?: boolean;
}

/**
 * Автосохранение формы шага мастера: дебаунс 1 с после последнего ввода
 * плюс интервал раз в 2 минуты. Ручной кнопки «Сохранить» на шагах больше нет.
 *
 * Сохраняет также при размонтировании — иначе переход на следующий шаг
 * внутри секунды после правки терял бы её.
 */
export function useAutosave<T>({ data, ready, save, enabled = true }: Options<T>) {
  // Слепок того, что уже лежит на сервере. null — форма ещё не готова.
  const baseline = useRef<string | null>(null);
  const latest = useRef<T>(data);
  const saveFn = useRef(save);
  const timer = useRef<number | undefined>(undefined);
  const inFlight = useRef(false);

  latest.current = data;
  saveFn.current = save;

  const flush = useCallback(async () => {
    if (!enabled || baseline.current === null || inFlight.current) return;

    const payload = JSON.stringify(latest.current);
    if (payload === baseline.current) return; // менять нечего

    inFlight.current = true;
    markSaving();
    try {
      await saveFn.current(latest.current);
      baseline.current = payload;
      markSaved();
    } catch (e) {
      markSaveError(extractError(e));
    } finally {
      inFlight.current = false;
    }
  }, [enabled]);

  /**
   * Слепок текущего состояния формы.
   *
   * Формы собирают payload заново на каждый рендер, поэтому по ссылке
   * `data` всегда «новый» объект. Сравнивать надо содержимое — и делать
   * это один раз за рендер: раньше JSON.stringify вызывался дважды, а
   * эффект дебаунса срабатывал на каждый рендер и снимал-ставил таймер
   * даже когда в форме ничего не менялось.
   */
  const snapshot = useMemo(() => JSON.stringify(data), [data]);

  // Первичный слепок — как только данные пришли с сервера.
  useEffect(() => {
    if (ready && baseline.current === null) {
      baseline.current = snapshot;
    }
  }, [ready, snapshot]);

  // Дебаунс на каждое изменение.
  useEffect(() => {
    if (!enabled || !ready || baseline.current === null) return;
    if (snapshot === baseline.current) return;

    window.clearTimeout(timer.current);
    timer.current = window.setTimeout(flush, DEBOUNCE_MS);
    return () => window.clearTimeout(timer.current);
  }, [snapshot, enabled, ready, flush]);

  // Периодическое сохранение — на случай, если дебаунс не сработал
  // (например, вкладка была неактивна).
  useEffect(() => {
    if (!enabled) return;
    const id = window.setInterval(() => { void flush(); }, INTERVAL_MS);
    return () => window.clearInterval(id);
  }, [enabled, flush]);

  // Уход со страницы/шага — досохраняем несохранённое.
  useEffect(() => {
    return () => {
      window.clearTimeout(timer.current);
      void flush();
    };
  }, [flush]);

  return { flush };
}
