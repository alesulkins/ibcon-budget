import type { User } from '../types';

const TOKEN_KEY = 'token';
const USER_KEY = 'user';
const REMEMBER_KEY = 'remember';
const LAST_ACTIVITY_KEY = 'last_activity';
const SAVED_EMAIL_KEY = 'saved_email';

/**
 * Автовыход при бездействии — 60 минут (ТЗ 3.8 п.6).
 * Дублирует серверную проверку в middleware.Auth: клиент выкидывает
 * пользователя сам, не дожидаясь ответа 401 на следующий запрос.
 */
export const INACTIVITY_LIMIT_MS = 60 * 60 * 1000;

/** Причина, по которой пользователя вернуло на форму логина. */
export type LogoutReason = 'timeout' | 'disabled' | 'manual';

interface AuthState {
  token: string | null;
  user: User | null;
}

/**
 * Где лежит токен:
 *   «Запомнить меня» включено → localStorage, переживает закрытие браузера
 *   выключено                 → sessionStorage, живёт до конца вкладки
 * Срок жизни самого токена задаёт сервер (90 дней против JWT_EXPIRY_HOURS).
 */
function tokenStore(): Storage {
  return localStorage.getItem(REMEMBER_KEY) === '1' ? localStorage : sessionStorage;
}

function readBoth(key: string): string | null {
  return sessionStorage.getItem(key) ?? localStorage.getItem(key);
}

function removeBoth(key: string) {
  sessionStorage.removeItem(key);
  localStorage.removeItem(key);
}

export function getToken(): string | null {
  return readBoth(TOKEN_KEY);
}

function load(): AuthState {
  const token = getToken();
  const raw = readBoth(USER_KEY);
  let user: User | null = null;
  if (raw && raw !== 'undefined' && raw !== 'null') {
    try {
      user = JSON.parse(raw) as User;
    } catch {
      clearAuth();
      return { token: null, user: null };
    }
  }
  return { token, user };
}

export function getAuth(): AuthState {
  return load();
}

export function setAuth(token: string, user: User, remember = false) {
  // Порядок важен: сначала флаг, потом запись — tokenStore() читает флаг.
  clearAuth({ keepSavedEmail: true });
  if (remember) {
    localStorage.setItem(REMEMBER_KEY, '1');
    localStorage.setItem(SAVED_EMAIL_KEY, user.email);
  } else {
    localStorage.removeItem(SAVED_EMAIL_KEY);
  }

  const store = tokenStore();
  store.setItem(TOKEN_KEY, token);
  store.setItem(USER_KEY, JSON.stringify(user));
  touchActivity();
}

export function clearAuth(opts?: { keepSavedEmail?: boolean }) {
  removeBoth(TOKEN_KEY);
  removeBoth(USER_KEY);
  removeBoth(LAST_ACTIVITY_KEY);
  localStorage.removeItem(REMEMBER_KEY);
  if (!opts?.keepSavedEmail) {
    localStorage.removeItem(SAVED_EMAIL_KEY);
  }
}

/** Email последнего входа с «Запомнить меня» — для подстановки в форму. */
export function savedEmail(): string {
  return localStorage.getItem(SAVED_EMAIL_KEY) ?? '';
}

export function isLoggedIn(): boolean {
  return !!getToken();
}

export function currentUser(): User | null {
  return load().user;
}

export function hasRole(...roles: string[]): boolean {
  const u = currentUser();
  if (!u) return false;
  return roles.includes(u.role);
}

/**
 * Отметка активности. Живёт в localStorage, чтобы работа в одной вкладке
 * продлевала сессию во всех остальных — иначе фоновая вкладка выкинула бы
 * пользователя посреди работы.
 */
export function touchActivity() {
  localStorage.setItem(LAST_ACTIVITY_KEY, String(Date.now()));
}

export function lastActivity(): number {
  const raw = localStorage.getItem(LAST_ACTIVITY_KEY);
  const ts = raw ? Number(raw) : NaN;
  return Number.isFinite(ts) ? ts : Date.now();
}

export function idleMs(): number {
  return Date.now() - lastActivity();
}

export function isIdleExpired(): boolean {
  return isLoggedIn() && idleMs() > INACTIVITY_LIMIT_MS;
}

/** Выход с возвратом на форму логина и объяснением причины. */
export function logout(reason: LogoutReason = 'manual') {
  clearAuth({ keepSavedEmail: true });
  const suffix = reason === 'manual' ? '' : `?reason=${reason}`;
  window.location.replace(`/login${suffix}`);
}
