import dayjs from 'dayjs';

export function fmtDate(d: string | null | undefined): string {
  if (!d) return '—';
  return dayjs(d).format('DD.MM.YYYY');
}

export function fmtDateTime(d: string | null | undefined): string {
  if (!d) return '—';
  return dayjs(d).format('DD.MM.YYYY HH:mm');
}

export function fmtMoney(n: number | null | undefined): string {
  if (n == null) return '—';
  return new Intl.NumberFormat('ru-RU', {
    maximumFractionDigits: 0,
  }).format(n) + ' ₽';
}

export function fmtPct(n: number | null | undefined): string {
  if (n == null) return '—';
  return n.toFixed(1) + ' %';
}

/**
 * Разделитель разрядов для InputNumber. Всегда идёт в паре с
 * thousandParser: без парсера antd не сможет разобрать обратно строку
 * с пробелами, и введённое значение портится.
 */
export function thousandFormatter(v: string | number | undefined | null): string {
  if (v === undefined || v === null || v === '') return '';
  return `${v}`.replace(/\B(?=(\d{3})+(?!\d))/g, ' ');
}

export function thousandParser(v: string | undefined): number {
  const n = Number((v ?? '').replace(/\s/g, ''));
  return Number.isFinite(n) ? n : 0;
}

/**
 * Подпись месяца проекта: 0 — первый месяц. Формат «дек 26»
 * (локаль dayjs выставлена в main.tsx).
 *
 * Единая точка для всех экранов мастера: раньше часть форм рисовала
 * «М1/М2», часть — свои даты.
 */
export function monthLabel(startDate: string | null | undefined, idx: number): string {
  if (!startDate) return `М${idx + 1}`;
  const d = dayjs(startDate).add(idx, 'month');
  if (!d.isValid()) return `М${idx + 1}`;
  return d.format('MMM YY');
}
