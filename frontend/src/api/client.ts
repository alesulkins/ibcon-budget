import axios from 'axios';

const API_BASE = import.meta.env.VITE_API_URL ?? '/api/v1';

export const client = axios.create({
  baseURL: API_BASE,
  timeout: 30_000,
});

// Прикрепляем JWT из localStorage
client.interceptors.request.use((cfg) => {
  const token = localStorage.getItem('token');
  if (token) cfg.headers.Authorization = `Bearer ${token}`;
  return cfg;
});

// Если 401 — разлогиниваем
client.interceptors.response.use(
  (r) => r,
  (err) => {
    if (err.response?.status === 401) {
      localStorage.removeItem('token');
      localStorage.removeItem('user');
      window.location.href = '/login';
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
