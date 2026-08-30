import { useLayoutEffect, useRef, useState } from 'react';
import { SCROLL_ROOT_ID } from './useScrollRestore';

/**
 * Id блока «Личный кабинет» внизу сайдбара (AppLayout.tsx) — единственный
 * внешний ориентир этого хука. Заведён здесь, а не в AppLayout, чтобы
 * хук и его ориентир были видны рядом.
 */
export const SIDER_FOOTER_ID = 'ibcon-sider-footer';

/**
 * Воздух между низом таблицы и линией ЛК — таблица должна не просто не
 * заходить за эту линию, а оставлять под ней заметное пустое место.
 * Значение то же, что у отступа рабочей области (Content, padding: 24
 * в AppLayout) — воздух снизу таблицы вторит воздуху сверху страницы.
 */
const BOTTOM_GAP = 24;

/** Ниже этой высоты таблице сжиматься не даём — пары строк не видно. */
const MIN_HEIGHT = 220;

/**
 * Высота, которую нужно отдать `Table.scroll.y`, чтобы вся таблица —
 * шапка и прокручиваемое тело вместе — не переросла вниз линию над
 * блоком «Личный кабинет» в сайдбаре. Большие таблицы (реестр,
 * пользователи, история) не должны продавливать страницу ниже этой
 * линии — вместо этого включается их собственная прокрутка.
 *
 * `scroll.y` ограничивает только ТЕЛО таблицы: antd рисует шапку особым
 * элементом (`.ant-table-thead`) рядом с прокручиваемым телом, а не
 * внутри него. Отдать в `scroll.y` всё доступное место — значит вылезти
 * вниз ровно на высоту шапки, что здесь и происходило. Поэтому мы её
 * измеряем и вычитаем.
 *
 * `reserveBottom` — что рисуется НИЖЕ прокручиваемой части, но внутри
 * того же измеряемого блока и в возвращённую высоту входить не должно
 * (например, подвал пагинации antd).
 *
 * Пересчитывается по изменению размера окна. Первое измерение — в
 * useLayoutEffect, до отрисовки кадра, чтобы не мелькнуть неверной
 * высотой. Позиция ref-элемента не зависит от его же высоты (её задают
 * только элементы над ним), поэтому измерение корректно даже пока
 * таблица ещё не сжата и временно выше окна. `<thead>` в разметке
 * antd есть независимо от того, задан ли уже `scroll.y`, поэтому мерить
 * его можно сразу, без отдельного прохода после первой отрисовки —
 * но на всякий случай кадром позже пересчитываем ещё раз: если высота
 * шапки почему-то не совпала, это самокорректируется без миганий.
 */
export function useFillToSiderFooter<T extends HTMLElement>(reserveBottom = 0) {
  const ref = useRef<T>(null);
  const [height, setHeight] = useState<number>();

  useLayoutEffect(() => {
    function recompute() {
      const el = ref.current;
      if (!el) return;

      // На телефоне панель выезжает поверх страницы и, пока закрыта, её
      // блока «Личный кабинет» в разметке нет вовсе. Ориентир тогда —
      // нижний край окна: линии, ниже которой нельзя, просто не
      // существует. Закрытая панель даёт нулевую высоту — на неё
      // ориентироваться тоже нельзя, иначе таблица схлопнется.
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
    return () => {
      cancelAnimationFrame(raf);
      window.removeEventListener('resize', recompute);
    };
  }, [reserveBottom]);

  return [ref, height] as const;
}
