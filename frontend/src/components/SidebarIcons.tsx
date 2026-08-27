/**
 * Иконки навигации — контурные, одной толщины линии.
 *
 * Свои вместо набора @ant-design/icons: там знаки нарисованы заливкой и
 * разной плотности, из-за чего в столбце выглядели неровно. Здесь у всех
 * одна сетка 16×16 и один штрих 1.4.
 */

const box = {
  width: 16,
  height: 16,
  display: 'block',
  flexShrink: 0,
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
    <svg viewBox="0 0 16 16" style={box} aria-hidden="true">
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
    <svg viewBox="0 0 16 16" style={box} aria-hidden="true">
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
    <svg viewBox="0 0 16 16" style={box} aria-hidden="true">
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
    <svg viewBox="0 0 16 16" style={box} aria-hidden="true">
      <g {...stroke}>
        <circle cx="8" cy="8" r="6" />
        <path d="M8 4.5V8l2.4 1.6" />
      </g>
    </svg>
  );
}
