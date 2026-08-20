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
