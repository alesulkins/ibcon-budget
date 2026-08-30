import { useEffect } from 'react';
import { ConfigProvider, theme as antdTheme } from 'antd';
import ruRU from 'antd/locale/ru_RU';
import App from '../App';
import { useUISettings, FONT_SIZE_PX } from '../store/uiSettings';
import {
  BRAND_WHITE, PAGE_BG, FONT_UI, RADIUS, RADIUS_LG, RADIUS_SM,
  TEXT, TEXT_SOFT, LINE,
  CELL_PAD_X, CELL_PAD_Y, MENU_ICON_GAP, MENU_FONT_SIZE,
  HOVER_DARK, MENU_SELECTED,
} from '../theme';

/**
 * Тёмная палитра. Не инверсия светлой: цвета подобраны так, чтобы
 * фирменный тон остался узнаваемым, а текст не «звенел» на чёрном.
 * Рабочая область чуть светлее фона карточек — та же логика, что в
 * светлой теме, только наоборот.
 */
const DARK = {
  text: '#E6EDF0',
  textSoft: '#9FB3BC',
  line: 'rgba(230, 237, 240, 0.12)',
  hover: 'rgba(230, 237, 240, 0.06)',
};

/**
 * Поверхности тёмной темы выводятся из ВЫБРАННОГО цвета, а не берутся из
 * фиксированной палитры: иначе фиолетовый сайдбар стоял бы на синем
 * фоне. Коэффициенты подобраны по фирменной паре #183E4D → #142127 и
 * #1B2A31: рабочая область темнее выбранного цвета, карточки чуть
 * светлее её — та же лесенка, что в светлой теме, только вниз.
 */
const DARK_BG_MIX = -0.62; // рабочая область
const DARK_SURFACE_MIX = -0.5; // карточки, поля, всплывающие панели

/**
 * Оттенок фирменного цвета: k > 0 светлее, k < 0 темнее.
 *
 * Нужен растяжке сайдбара и экрану входа: раньше их оттенки были
 * вписаны литералами и не менялись вместе с выбранным цветом.
 */
function shade(hex: string, k: number): string {
  const m = /^#?([0-9a-f]{6})$/i.exec(hex.trim());
  if (!m) return hex; // не шестизначный hex — оставляем как есть
  const n = parseInt(m[1], 16);
  const mix = (c: number) => {
    const target = k > 0 ? 255 : 0;
    return Math.round(c + (target - c) * Math.abs(k));
  };
  const r = mix((n >> 16) & 255);
  const g = mix((n >> 8) & 255);
  const b = mix(n & 255);
  return `#${((r << 16) | (g << 8) | b).toString(16).padStart(6, '0')}`;
}

/** rgba из hex — для полупрозрачных подложек поверх фирменного цвета. */
function alpha(hex: string, a: number): string {
  const m = /^#?([0-9a-f]{6})$/i.exec(hex.trim());
  if (!m) return hex;
  const n = parseInt(m[1], 16);
  return `rgba(${(n >> 16) & 255}, ${(n >> 8) & 255}, ${n & 255}, ${a})`;
}

/**
 * Оформление приложения: тема, размер шрифта и фирменные цвета из
 * персональных настроек пользователя.
 *
 * Живёт отдельным компонентом, а не в main.tsx, потому что настройки
 * приходят из учётки — их надо прочитать хуком, а хук требует, чтобы
 * компонент был внутри провайдеров.
 */
export default function ThemedApp() {
  const { fontSize, theme, brandColor, noticeColor } = useUISettings();
  const dark = theme === 'dark';
  const basePx = FONT_SIZE_PX[fontSize];

  // Светлая тема: фон и карточки остаются фирменными бежевыми — решение
  // владельца. Меняется акцент (сайдбар, кнопки, выделение), а не бумага
  // под содержимым.
  const white = dark ? shade(brandColor, DARK_SURFACE_MIX) : BRAND_WHITE;
  const bg = dark ? shade(brandColor, DARK_BG_MIX) : PAGE_BG;
  const text = dark ? DARK.text : TEXT;
  const textSoft = dark ? DARK.textSoft : TEXT_SOFT;
  const line = dark ? DARK.line : LINE;
  // Подсветка при наведении — тот же цвет, взятый почти прозрачным:
  // своего серого в оформлении нет, и на фиолетовом сине-зелёная
  // подсветка выглядела бы чужой.
  const hover = dark ? DARK.hover : alpha(brandColor, 0.055);

  /**
   * Часть оформления живёт в index.css — правилами, которые нельзя
   * задать инлайновым стилем. Синхронизируем их переменные с выбранными
   * настройками: иначе тема сменилась бы только у компонентов antd, а
   * наши правила остались бы светлыми.
   */
  useEffect(() => {
    const root = document.documentElement.style;
    root.setProperty('--ibcon-brand', brandColor);
    root.setProperty('--ibcon-white', white);
    root.setProperty('--ibcon-bg', bg);
    root.setProperty('--ibcon-line', line);
    root.setProperty('--ibcon-muted', textSoft);
    root.setProperty('--ibcon-text', text);
    root.setProperty('--ibcon-hover', hover);
    root.setProperty('--ibcon-notice', noticeColor);

    // Оттенки фирменного цвета: растяжка сайдбара и шапка входа. Раньше
    // они были вписаны литералами и не менялись вместе с цветом.
    //
    // Имена намеренно не «light»/«dark»: рядом уже есть флаг темы `dark`,
    // и одноимённая переменная затеняла бы его — все проверки ниже
    // читали бы строку с цветом, а она истинна всегда, и светлая тема
    // получала бы тёмные подписи.
    const brandLight = shade(brandColor, 0.18);
    const brandDark = shade(brandColor, -0.28);
    root.setProperty('--ibcon-brand-light', brandLight);
    // Ступень между обычным и наведённым состоянием кнопки.
    root.setProperty('--ibcon-brand-hover', shade(brandColor, 0.3));
    root.setProperty('--ibcon-brand-dark', brandDark);
    root.setProperty('--ibcon-sider-gradient',
      `linear-gradient(170deg, ${alpha(brandLight, 0.92)} 0%, `
      + `${alpha(brandColor, 0.95)} 45%, ${alpha(brandDark, 0.97)} 100%)`);

    // Шаги мастера. В тёмной теме подложка пройденного шага и подписи
    // строились из тёмного фирменного цвета и почти сливались с фоном —
    // там берём светлые тона.
    root.setProperty('--ibcon-step-done-bg',
      dark ? 'rgba(230, 237, 240, 0.16)' : alpha(brandColor, 0.12));
    root.setProperty('--ibcon-step-border',
      dark ? 'rgba(230, 237, 240, 0.28)' : alpha(brandColor, 0.18));
    root.setProperty('--ibcon-step-todo', dark ? DARK.textSoft : '#8c9aa0');
    root.setProperty('--ibcon-step-label', dark ? DARK.text : '#595959');
    // На тёмном фоне фирменный цвет как подпись не читается — берём
    // светлый тон той же гаммы.
    root.setProperty('--ibcon-step-active-label',
      dark ? shade(brandColor, 0.62) : brandColor);
    // Шапка страницы. В тёмной теме её подложка выводится из выбранного
    // цвета тем же способом, что фон и карточки (коэффициент чуть
    // светлее фона, чтобы шапка читалась как отдельный слой). В светлой
    // теме шапка остаётся белой — решение владельца.
    root.setProperty('--ibcon-header-bg',
      dark ? alpha(shade(brandColor, -0.56), 0.86) : 'rgba(255, 255, 255, 0.86)');
    root.setProperty('--ibcon-scrollbar',
      dark ? 'rgba(230, 237, 240, 0.24)' : alpha(brandColor, 0.28));
    // Базовый кегль: от него antd считает свои размеры, а наши
    // относительные единицы — свои.
    root.setProperty('--ibcon-font-size', `${basePx}px`);
    // Тема как атрибут — по нему index.css правит то, что не выражается
    // переменными (например, инверсию теней).
    document.documentElement.dataset.theme = theme;
  }, [brandColor, white, bg, line, textSoft, text, hover, noticeColor, basePx, theme, dark]);

  return (
    <ConfigProvider
      locale={ruRU}
      theme={{
        // cssVar переводит antd на CSS-переменные: стили считаются один
        // раз, а не пересобираются рантайм-движком на каждую новую
        // комбинацию компонентов. hashed: false убирает хеш-классы —
        // меньше работы стилевому движку и чище инспектор.
        cssVar: { key: 'ibcon' },
        hashed: false,
        algorithm: dark ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm,
        token: {
          colorPrimary: brandColor,
          // Поля ввода и всплывающие поверхности — фирменный белый.
          // Плиты карточек и таблиц убраны в index.css: выбранное
          // оформление ставит содержимое прямо на рабочую область.
          colorBgContainer: white,
          colorBgElevated: white,
          // Совпадает с --ibcon-bg: иначе на краях страницы виден стык
          // двух почти одинаковых фонов.
          colorBgLayout: bg,
          colorText: text,
          colorTextHeading: text,
          colorTextSecondary: textSoft,
          colorTextDescription: textSoft,
          colorTextPlaceholder: textSoft,
          colorBorder: line,
          colorBorderSecondary: line,
          colorSplit: line,
          fontFamily: FONT_UI,
          borderRadius: RADIUS,
          borderRadiusLG: RADIUS_LG,
          borderRadiusSM: RADIUS_SM,
          // Размер шрифта интерфейса. Остальные кегли antd считает от
          // него сам, поэтому вёрстка не расползается: меняется текст, а
          // не масштаб страницы.
          fontSize: basePx,
        },
        /**
         * Токены компонентов. Всё, что здесь, задавалось бы правилами
         * в index.css — но у antd v6 селекторы обёрнуты в :where(), у
         * наших правил та же специфичность, а его стили попадают в
         * <head> позже. При равенстве выигрывает последний, поэтому
         * CSS-переопределения молча не работали. Токены в этот спор не
         * вступают: antd генерирует стиль сразу с нашими значениями.
         *
         * Числа — из theme.ts, менять там.
         */
        components: {
          Table: {
            // Таблицы стоят с size="small", поэтому важны именно SM.
            cellPaddingBlockSM: CELL_PAD_Y,
            cellPaddingInlineSM: CELL_PAD_X,
            cellPaddingBlock: CELL_PAD_Y,
            cellPaddingInline: CELL_PAD_X,
            cellPaddingBlockMD: CELL_PAD_Y,
            cellPaddingInlineMD: CELL_PAD_X,
            // Шапка того же цвета, что рабочая область.
            headerBg: bg,
            headerColor: textSoft,
            headerSortActiveBg: bg,
            headerSortHoverBg: bg,
            rowHoverBg: hover,
            borderColor: line,
          },
          Menu: {
            // Скруглённая плашка под наведением и под выбранным пунктом.
            itemBorderRadius: RADIUS,
            darkItemHoverBg: HOVER_DARK,
            darkItemSelectedBg: MENU_SELECTED,
            darkItemBg: 'transparent',
            darkSubMenuItemBg: 'transparent',
            iconMarginInlineEnd: MENU_ICON_GAP,
            fontSize: MENU_FONT_SIZE,
          },
          Notification: {
            // Уведомления красятся своим цветом: их замечают краем глаза,
            // и он может отличаться от фирменного.
            colorInfo: noticeColor,
          },
        },
      }}
    >
      <App />
    </ConfigProvider>
  );
}
