import { useEffect } from 'react';
import { useLocation } from 'react-router-dom';

const KEY_PREFIX = 'scroll:';

/**
 * Идентификатор прокручиваемой панели приложения.
 *
 * Документ не прокручивается вовсе (см. index.css): окно фиксировано по
 * высоте, а прокрутка отдана рабочей области. Поэтому позицию читаем и
 * восстанавливаем у неё, а не у window.
 */
export const SCROLL_ROOT_ID = 'ibcon-scroll-root';

function scrollRoot(): HTMLElement | null {
  return document.getElementById(SCROLL_ROOT_ID);
}

/**
 * Переносит запомненную позицию прокрутки на другой адрес.
 *
 * Нужно при переключении между версиями бюджета: адрес меняется, а формы
 * под ним те же, и сравнивать их удобно только с одного и того же места.
 * Позицию берём не из хранилища, а у живой панели — там она свежее: в
 * хранилище значение попадает по событию прокрутки, а уйти со страницы
 * можно и раньше.
 */
export function carryScrollTo(pathname: string) {
  const el = scrollRoot();
  if (!el) return;
  try {
    sessionStorage.setItem(KEY_PREFIX + pathname, String(el.scrollTop));
  } catch { /* приватный режим — просто откроется сверху */ }
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
    //
    // Сразу после навигации контента нет вовсе: версия бюджета грузится
    // запросом, панель схлопывается, и браузер сам сбрасывает прокрутку
    // в ноль. Пары кадров тут мало — ждём появления высоты наблюдателем
    // за размером, с ограничением по времени, чтобы не держать его
    // вечно на странице, которая так и осталась короткой.
    let observer: ResizeObserver | undefined;
    let giveUp: number | undefined;

    if (saved > 0) {
      const tryScroll = () => {
        const reachable = el.scrollHeight - el.clientHeight;
        if (reachable < saved) return false;
        el.scrollTop = saved;
        return true;
      };

      if (!tryScroll()) {
        observer = new ResizeObserver(() => {
          if (tryScroll()) stopWaiting();
        });
        observer.observe(el);
        giveUp = window.setTimeout(() => {
          // Страница оказалась короче, чем была: так бывает при
          // переключении между версиями бюджета — у одной есть
          // предупреждение в шапке, у другой нет. Докручиваем до конца:
          // это ближе к искомому месту, чем прыжок в начало.
          const reachable = el.scrollHeight - el.clientHeight;
          if (reachable > 0 && el.scrollTop === 0) el.scrollTop = reachable;
          stopWaiting();
        }, 3000);
      }
    }

    function stopWaiting() {
      observer?.disconnect();
      observer = undefined;
      if (giveUp !== undefined) {
        clearTimeout(giveUp);
        giveUp = undefined;
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
