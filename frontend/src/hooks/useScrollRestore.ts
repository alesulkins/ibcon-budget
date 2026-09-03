import { useCallback, useEffect, useLayoutEffect, useRef } from 'react';
import { useLocation } from 'react-router-dom';

const KEY_PREFIX = 'scroll:';

// Идентификатор прокручиваемой панели приложения. Документ не прокручивается
// вовсе (см. index.css): окно фиксировано по высоте, а прокрутка отдана
// рабочей области.
export const SCROLL_ROOT_ID = 'ibcon-scroll-root';

function scrollRoot(): HTMLElement | null {
  return document.getElementById(SCROLL_ROOT_ID);
}

// Переносит запомненную позицию прокрутки на другой адрес.
export function carryScrollTo(pathname: string) {
  const el = scrollRoot();
  if (!el) return;
  try {
    sessionStorage.setItem(KEY_PREFIX + pathname, String(el.scrollTop));
  } catch { /* приватный режим — просто откроется сверху */ }
}

// Удерживает уровень прокрутки при смене шага мастера. Полоса шагов
// прилипшая, а формы под ней разной высоты.
export function useHoldScroll(dep: unknown) {
  const kept = useRef<number | null>(null);

  // Уровень снимаем в обработчике клика, а не в эффекте: к моменту
  // эффекта содержимое уже сменилось, а прокрутка — уже сбита.
  const hold = useCallback(() => {
    const el = scrollRoot();
    kept.current = el ? el.scrollTop : null;
  }, []);

  useLayoutEffect(() => {
    const want = kept.current;
    kept.current = null;
    const el = scrollRoot();
    if (!el || want === null || want <= 0) return;

    let observer: ResizeObserver | undefined;
    let giveUp: number | undefined;

    const stopWaiting = () => {
      observer?.disconnect();
      observer = undefined;
      if (giveUp !== undefined) {
        clearTimeout(giveUp);
        giveUp = undefined;
      }
    };

    const tryScroll = () => {
      if (el.scrollHeight - el.clientHeight < want) return false;
      el.scrollTop = want;
      return true;
    };

    if (!tryScroll()) {
      observer = new ResizeObserver(() => {
        if (tryScroll()) stopWaiting();
      });
      observer.observe(el);
      // Наблюдаем и за содержимым: у самой панели размер задан
      // раскладкой и не меняется — растёт то, что внутри неё.
      if (el.firstElementChild) observer.observe(el.firstElementChild);
      giveUp = window.setTimeout(() => {
        // Форма на новом шаге просто короче прежней — докручиваем до
        // конца: это ближе к прежнему месту, чем начало страницы.
        const reachable = el.scrollHeight - el.clientHeight;
        if (reachable > 0) el.scrollTop = Math.min(want, reachable);
        stopWaiting();
      }, 1500);
    }

    return stopWaiting;
  }, [dep]);

  return hold;
}

/**
 * Запоминает позицию прокрутки для каждого адреса и восстанавливает её
 * при возврате — чтобы уход в «Справочники» и обратно не выбрасывал
 * пользователя в начало длинной формы.
 */
export function useScrollRestore() {
  const { pathname } = useLocation();

  useEffect(() => {
    const key = KEY_PREFIX + pathname;
    const el = scrollRoot();
    if (!el) return;

    const save = () => {
      try {
        sessionStorage.setItem(key, String(el.scrollTop));
      } catch { /* приватный режим — не критично */ }
    };

    let saved = 0;
    try {
      saved = Number(sessionStorage.getItem(key) ?? '0');
    } catch { /* нет доступа к хранилищу — начнём сверху */ }

    // Восстановление ждёт, пока панель дорастёт до нужной высоты.
    let frame: number | undefined;
    const deadline = performance.now() + 3000;

    if (saved > 0) {
      const tryScroll = () => {
        const reachable = el.scrollHeight - el.clientHeight;
        if (reachable < saved) return false;
        el.scrollTop = saved;
        return true;
      };

      const step = () => {
        if (tryScroll()) {
          frame = undefined;
          return;
        }
        if (performance.now() > deadline) {
          // Страница оказалась короче, чем была: так бывает при переключении
          // между версиями бюджета — у одной есть предупреждение в шапке, у
          // другой нет.
          const reachable = el.scrollHeight - el.clientHeight;
          if (reachable > 0 && el.scrollTop === 0) {
            el.scrollTop = Math.min(saved, reachable);
          }
          frame = undefined;
          return;
        }
        frame = requestAnimationFrame(step);
      };
      frame = requestAnimationFrame(step);
    }

    function stopWaiting() {
      if (frame !== undefined) {
        cancelAnimationFrame(frame);
        frame = undefined;
      }
    }

    el.addEventListener('scroll', save, { passive: true });
    return () => {
      // Сохраняем только осмысленную позицию: при уходе со страницы
      // контент уже размонтирован, панель схлопнулась, и scrollTop
      // сброшен браузером в ноль — записав его, мы бы затёрли
      // настоящую позицию, которую пользователь оставил.
      if (el.scrollTop > 0) save();
      stopWaiting();
      el.removeEventListener('scroll', save);
    };
  }, [pathname]);
}
