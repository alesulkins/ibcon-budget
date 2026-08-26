import axios from 'axios';
import { getToken, touchActivity, clearAuth } from '../store/auth';

const API_BASE = import.meta.env.VITE_API_URL ?? '/api/v1';

export const client = axios.create({
  baseURL: API_BASE,
  timeout: 30_000,
});

// Прикрепляем JWT. Токен может лежать в sessionStorage (обычный вход)
// или в localStorage («Запомнить меня») — getToken() знает, где искать.
client.interceptors.request.use((cfg) => {
  const token = getToken();
  if (token) cfg.headers.Authorization = `Bearer ${token}`;
  return cfg;
});

client.interceptors.response.use(
  (r) => {
    // Успешный запрос — признак активности: продлевает и клиентский
    // таймер бездействия, и серверный last_activity.
    touchActivity();
    return r;
  },
  (err) => {
    if (err.response?.status === 401) {
      // Сервер различает причины: истёкшая по бездействию сессия,
      // отключённая учётка, невалидный токен.
      const code = err.response?.data?.code;
      const reason =
        code === 'session_timeout' ? 'timeout' :
        code === 'account_disabled' ? 'disabled' : '';

      clearAuth({ keepSavedEmail: true });
      if (window.location.pathname !== '/login') {
        window.location.replace(reason ? `/login?reason=${reason}` : '/login');
      }
    }
    return Promise.reject(err);
  },
);

export function extractError(err: unknown): string {
  if (axios.isAxiosError(err)) {
    return err.response?.data?.error ?? err.message;
  }
  return String(err);
}
