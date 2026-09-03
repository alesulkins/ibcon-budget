// Фирменные цвета IBCON — единственное место, где они заданы.

/** Фирменный цвет компании. */
export const BRAND = '#183E4D';

/** Фирменный белый: карточки, таблицы, модалы, поля ввода. */
export const BRAND_WHITE = '#F2EDEB';

/**
 * Фон рабочей области — фирменный белый, приглушённый на несколько
 * процентов. Карточки на нём читаются как приподнятые, при этом оба
 * тона остаются из одной тёплой гаммы.
 */
export const PAGE_BG = '#F2EDEB';

// Оттенки фирменного цвета для растяжки сайдбара.
export const BRAND_LIGHT = '#22576C';
export const BRAND_DARK = '#0F2832';

/** Шрифт интерфейса. */
export const FONT_UI =
  "'IBM Plex Sans', system-ui, -apple-system, 'Segoe UI', sans-serif";

/**
 * Шрифт чисел: моноширинная пара к основному, поэтому цифры совпадают
 * по рисунку и высоте. Разряды в колонке встают друг под друга.
 */
export const FONT_NUM =
  "'IBM Plex Mono', ui-monospace, SFMono-Regular, Menlo, monospace";

// Скругления. Крупное — плиты и модалы, базовое — кнопки и поля.
export const RADIUS_LG = 8;
export const RADIUS = 6;
export const RADIUS_SM = 4;

/** Цвет текста и подписей — из пульта оформления. */
export const TEXT = '#16323D';
export const TEXT_SOFT = '#6A8089';

/** Единственная линия оформления: строки таблиц, шапки блоков, края. */
export const LINE = 'rgba(24, 62, 77, 0.10)';

// ═══ РУЧКИ ПОДГОНКИ ═══ Задаются здесь, а не в index.css.

/** ШИРИНА колонок таблиц: горизонтальный отступ в ячейке. */
export const CELL_PAD_X = 18;

/** ВЫСОТА строк таблиц: вертикальный отступ в ячейке. */
export const CELL_PAD_Y = 16;

/**
 * Отступ между иконкой и подписью в меню сайдбара.
 * Замерено: при подписи 13px «История изменений» занимает 123px, под
 * неё остаётся 148 минус этот отступ. Предел — 24, дальше обрежется.
 */
export const MENU_ICON_GAP = 16;

/** Размер подписи в меню. На 14px «История изменений» уже впритык. */
export const MENU_FONT_SIZE = 13;

/** Подсветка при наведении: на светлом фоне и на тёмной панели. */
export const HOVER_LIGHT = 'rgba(24, 62, 77, 0.055)';
export const HOVER_DARK = 'rgba(253, 249, 248, 0.10)';

/** Заливка выбранного пункта меню. */
export const MENU_SELECTED = 'rgba(253, 249, 248, 0.16)';

// Палитра статусов — приглушённая, из пульта оформления.
export const STATUS = {
  green: '#2F7D3A',
  amber: '#B07A12',
  blue: '#2D6A86',
  grey: '#6B7D84',
  orange: '#A8621A',
  red: '#9C2B2B',
  teal: '#2F7D78',
  violet: '#6A5A92',
} as const;

export type StatusColor = keyof typeof STATUS;
