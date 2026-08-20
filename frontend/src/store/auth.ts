import type { User } from '../types';

interface AuthState {
  token: string | null;
  user: User | null;
}

function load(): AuthState {
  const token = localStorage.getItem('token');
  const raw = localStorage.getItem('user');
  let user: User | null = null;
  if (raw && raw !== 'undefined' && raw !== 'null') {
    try {
      user = JSON.parse(raw) as User;
    } catch {
      localStorage.removeItem('token');
      localStorage.removeItem('user');
      return { token: null, user: null };
    }
  }
  return { token, user };
}

export function getAuth(): AuthState {
  return load();
}

export function setAuth(token: string, user: User) {
  localStorage.setItem('token', token);
  localStorage.setItem('user', JSON.stringify(user));
}

export function clearAuth() {
  localStorage.removeItem('token');
  localStorage.removeItem('user');
}

export function isLoggedIn(): boolean {
  return !!localStorage.getItem('token');
}

export function currentUser(): User | null {
  return load().user;
}

export function hasRole(...roles: string[]): boolean {
  const u = currentUser();
  if (!u) return false;
  return roles.includes(u.role);
}
