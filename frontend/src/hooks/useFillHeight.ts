import { useCallback, useLayoutEffect, useState } from 'react';
import { SCROLL_ROOT_ID } from './useScrollRestore';

// Id блока «Личный кабинет» внизу сайдбара (AppLayout.tsx) — единственный
// внешний ориентир этого хука.
export const SIDER_FOOTER_ID = 'ibcon-sider-footer';

// Воздух между низом таблицы и линией ЛК — таблица должна не просто не
// заходить за эту линию, а оставлять под ней заметное пустое место.
const BOTTOM_GAP = 24;

/** Ниже этой высоты таблице сжиматься не даём — пары строк не видно. */
const MIN_HEIGHT = 220;

/** Порог телефона — тот же, что у antd (md) и в каркасе приложения. */
const MOBILE_MAX = 768;

// Высота, которую нужно отдать `Table.scroll.y`, чтобы вся таблица — шапка и
// прокручиваемое тело вместе — не переросла вниз линию над блоком «Личный
// кабинет» в сайдбаре.
export function useFillToSiderFooter<T extends HTMLElement>(reserveBottom = 0) {
  // Элемент держим состоянием, а не обычным ref: страница сначала показывает
  // загрузку, и до прихода данных этого блока в разметке нет вовсе.
  const [el, setEl] = useState<T | null>(null);
  const ref = useCallback((node: T | null) => setEl(node), []);
  const [height, setHeight] = useState<number>();

  useLayoutEffect(() => {
    function recompute() {
      if (!el) return;

      // На телефоне высоту не ограничиваем вовсе: линии ЛК там нет, а окошко
      // в четверть экрана с собственной прокруткой внутри страницы — худшее
      // из решений.
      if (window.innerWidth < MOBILE_MAX) {
        setHeight(undefined);
        return;
      }

      // На телефоне панель выезжает поверх страницы и, пока закрыта, её
      // блока «Личный кабинет» в разметке нет вовсе.
      const footer = document.getElementById(SIDER_FOOTER_ID);
      const footerBox = footer?.getBoundingClientRect();
      const footerTop = footerBox && footerBox.height > 0
        ? footerBox.top
        : window.innerHeight;

      const top = el.getBoundingClientRect().top;
      let available = footerTop - top - BOTTOM_GAP - reserveBottom;

      // Если страница уже прокручена, верх таблицы оказывается ВЫШЕ окна,
      // и разница до линии ЛК выходит больше самой рабочей области —
      // таблица растягивалась на несколько экранов, и прокруток
      // становилось две. Ограничиваем высотой видимой части.
      const root = document.getElementById(SCROLL_ROOT_ID);
      if (root) {
        const visible = footerTop - root.getBoundingClientRect().top
          - BOTTOM_GAP - reserveBottom;
        available = Math.min(available, visible);
      }

      const thead = el.querySelector<HTMLElement>('.ant-table-thead');
      const theadHeight = thead ? thead.getBoundingClientRect().height : 0;

      setHeight(Math.max(MIN_HEIGHT, available - theadHeight));
    }
    recompute();
    const raf = requestAnimationFrame(recompute);
    window.addEventListener('resize', recompute);
    // Высота шапки таблицы меняется, когда приходят данные и подписи
    // колонок переносятся на две строки, — следим и за ней.
    const observer = new ResizeObserver(() => recompute());
    if (el) observer.observe(el);
    return () => {
      cancelAnimationFrame(raf);
      observer?.disconnect();
      window.removeEventListener('resize', recompute);
    };
  }, [el, reserveBottom]);

  return [ref, height] as const;
}
