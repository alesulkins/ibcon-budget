/**
 * Нормализация текстовых полей карточки проекта.
 *
 * Дублирует правила бэкенда (internal/projects/normalize.go) намеренно:
 * сервер — источник истины и приводит данные в любом случае, а фронт
 * показывает результат сразу при потере фокуса, чтобы пользователь видел,
 * во что превратился его ввод, ещё до сохранения.
 */

/**
 * Организационно-правовые формы — всегда заглавными, как бы их ни ввели.
 * Ключ в нижнем регистре.
 */
const LEGAL_FORMS: Record<string, string> = {
  'ооо': 'ООО',
  'оао': 'ОАО',
  'ао': 'АО',
  'зао': 'ЗАО',
  'ип': 'ИП',
  'пао': 'ПАО',
  'нко': 'НКО',
};

/**
 * Поднимает только первую букву слова. Без правил про правовые формы —
 * их нельзя применять к ФИО, иначе фамилия «Ип» стала бы «ИП».
 */
export function capitalizeWord(s: string): string {
  const t = (s ?? '').trim();
  if (!t) return t;
  return t.charAt(0).toLocaleUpperCase('ru') + t.slice(1);
}

/**
 * Первая буква заглавная; правовые формы поднимаются целиком в любом
 * месте строки: «ооо ромашка» → «ООО Ромашка».
 * Применяется к названиям, НЕ к ФИО.
 */
export function capitalizeFirst(s: string): string {
  const t = (s ?? '').trim();
  if (!t) return t;

  const words = t.split(/\s+/);
  let firstMeaningful = true;

  return words.map((w) => {
    // Правовую форму сверяем без окружающих кавычек и знаков.
    const trimmed = w.replace(/^["«»',.]+|["«»',.]+$/g, '');
    const up = LEGAL_FORMS[trimmed.toLocaleLowerCase('ru')];
    if (up) return w.replace(trimmed, up);

    // Первое «обычное» слово — с заглавной. Правовая форма впереди
    // не считается: в «ооо ромашка» подниматься должна «Ромашка».
    if (firstMeaningful) {
      firstMeaningful = false;
      return w.charAt(0).toLocaleUpperCase('ru') + w.slice(1);
    }
    return w;
  }).join(' ');
}

/**
 * Приводит ФИО к формату «Фамилия И.О.»:
 *
 *   «иванов и.о.»          → «Иванов И.О.»
 *   «иванов и. о.»         → «Иванов И.О.»
 *   «иванов иван олегович» → «Иванов И.О.»
 *   «иванов»               → «Иванов»
 *
 * Если разобрать не удалось, возвращаем ввод с заглавной первой буквой —
 * молча терять введённое нельзя.
 */
export function normalizeFullName(s: string): string {
  const t = (s ?? '').trim();
  if (!t) return t;

  const fields = t.replace(/\./g, '. ').split(/\s+/).filter(Boolean);
  if (fields.length === 0) return capitalizeWord(t);

  const surname = capitalizeWord(fields[0].replace(/\.$/, ''));
  if (!surname) return capitalizeWord(t);

  const initials = fields.slice(1)
    .map(f => f.replace(/\./g, ''))
    .filter(Boolean)
    .map(f => f.charAt(0).toLocaleUpperCase('ru'));

  if (initials.length === 0) return surname;
  return `${surname} ${initials.map(i => `${i}.`).join('')}`;
}

/**
 * Сокращает полное ФИО до «Фамилия И.О.».
 *
 * В системе два формата отображения: в личном кабинете и в управлении
 * пользователями ФИО показывается полностью (оно там и хранится), а во
 * всех остальных местах — сайдбар, шапка, история изменений, таблицы —
 * сокращённо. Преобразование делает эта функция, отдельного поля в БД
 * для короткой формы нет.
 *
 *   «Ярулина Анжела Ильгамовна» → «Ярулина А.И.»
 *   «Иванов Иван»               → «Иванов И.»
 *   «Иванов»                    → «Иванов»
 *   «Ярулина А.И.»              → «Ярулина А.И.» (уже сокращено)
 */
export function shortName(fullName: string | null | undefined): string {
  const t = (fullName ?? '').trim();
  if (!t) return '';
  return normalizeFullName(t);
}

/** Инициалы для аватара: «Ярулина Анжела Ильгамовна» → «ЯА». */
export function initials(fullName: string | null | undefined): string {
  const parts = (fullName ?? '').trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return '?';
  const first = parts[0].charAt(0);
  const second = parts.length > 1 ? parts[1].charAt(0) : '';
  return (first + second).toLocaleUpperCase('ru');
}
