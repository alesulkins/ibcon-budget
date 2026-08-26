import { Navigate } from 'react-router-dom';
import { isLoggedIn, isIdleExpired, clearAuth } from '../store/auth';
import { useInactivityLogout } from '../hooks/useInactivityLogout';
import type { ReactNode } from 'react';

export default function RequireAuth({ children }: { children: ReactNode }) {
  useInactivityLogout();

  // Вкладку могли открыть спустя час после последней активности —
  // тогда токен ещё лежит в хранилище, но сессия уже недействительна.
  if (isIdleExpired()) {
    clearAuth({ keepSavedEmail: true });
    return <Navigate to="/login?reason=timeout" replace />;
  }

  if (!isLoggedIn()) {
    return <Navigate to="/login" replace />;
  }
  return <>{children}</>;
}
