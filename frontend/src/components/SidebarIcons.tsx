/**
 * Иконки навигации — контурные, одной толщины линии.
 *
 * Свои вместо набора @ant-design/icons: там знаки нарисованы заливкой и
 * разной плотности, из-за чего в столбце выглядели неровно. Здесь у всех
 * одна сетка 16×16 и один штрих 1.4.
 */

/**
 * Отступ до подписи пункта меню задаём здесь.
 *
 * Токен antd `iconMarginInlineEnd` до этих знаков не достаёт: он
 * рассчитан на `.anticon` из набора @ant-design/icons и ставит поле
 * соседнему span по селектору `.anticon + span`. Наши знаки — обычные
 * svg без этого класса, поэтому подпись прижималась к иконке вплотную.
 */
const ICON_GAP = 10;

const box = {
  width: 16,
  height: 16,
  display: 'block',
  flexShrink: 0,
  marginInlineEnd: ICON_GAP,
} as const;

const stroke = {
  fill: 'none',
  stroke: 'currentColor',
  strokeWidth: 1.4,
  strokeLinecap: 'round',
  strokeLinejoin: 'round',
} as const;

/** Реестр проектов — таблица со строками. */
export function IconProjects() {
  return (
    <svg viewBox="0 0 16 16" className="ibcon-menu-icon" style={box} aria-hidden="true">
      <g {...stroke}>
        <rect x="1.5" y="2.5" width="13" height="11" rx="1.5" />
        <path d="M1.5 6h13M5.5 6v7.5" />
      </g>
    </svg>
  );
}

/** Справочники — раскрытая книга. */
export function IconReferences() {
  return (
    <svg viewBox="0 0 16 16" className="ibcon-menu-icon" style={box} aria-hidden="true">
      <g {...stroke}>
        <path d="M3 2.5h9a1 1 0 0 1 1 1v10a1 1 0 0 1-1 1H3z" />
        <path d="M3 2.5a1 1 0 0 0-1 1v10a1 1 0 0 0 1 1" />
        <path d="M5.5 6h5M5.5 9h3" />
      </g>
    </svg>
  );
}

/** Пользователи — фигура человека. */
export function IconUsers() {
  return (
    <svg viewBox="0 0 16 16" className="ibcon-menu-icon" style={box} aria-hidden="true">
      <g {...stroke}>
        <circle cx="8" cy="5.5" r="2.6" />
        <path d="M2.6 14c.6-2.9 2.8-4.3 5.4-4.3s4.8 1.4 5.4 4.3" />
      </g>
    </svg>
  );
}

/** История изменений — циферблат со стрелками. */
export function IconHistory() {
  return (
    <svg viewBox="0 0 16 16" className="ibcon-menu-icon" style={box} aria-hidden="true">
      <g {...stroke}>
        <circle cx="8" cy="8" r="6" />
        <path d="M8 4.5V8l2.4 1.6" />
      </g>
    </svg>
  );
}

/** Настройки: шестерёнка в том же контурном стиле, что остальные. */
export function IconSettings() {
  return (
    <svg viewBox="0 0 16 16" className="ibcon-menu-icon" style={box} aria-hidden="true">
      <g {...stroke}>
        <circle cx="8" cy="8" r="2.2" />
        <path d="M8 1.6v1.6M8 12.8v1.6M14.4 8h-1.6M3.2 8H1.6M12.5 3.5l-1.1 1.1M4.6 11.4l-1.1 1.1M12.5 12.5l-1.1-1.1M4.6 4.6L3.5 3.5" />
      </g>
    </svg>
  );
}
