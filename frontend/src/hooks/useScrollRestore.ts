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

    if (saved > 0) {
      // Контент подгружается асинхронно, и сразу после монтирования
      // панель ещё нулевой высоты — прокручивать некуда. Пробуем
      // несколько кадров подряд, пока высота не позволит.
      let attempts = 0;
      const tryScroll = () => {
        if (el.scrollTop === saved) return;
        const reachable = el.scrollHeight - el.clientHeight;
        if (reachable >= saved) {
          el.scrollTop = saved;
          return;
        }
        if (++attempts < 20) requestAnimationFrame(tryScroll);
      };
      requestAnimationFrame(tryScroll);
    }

    el.addEventListener('scroll', save, { passive: true });
    return () => {
      save();
      el.removeEventListener('scroll', save);
    };
  }, [pathname]);
}
