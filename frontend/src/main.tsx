import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import dayjs from 'dayjs';
import 'dayjs/locale/ru';
import ThemedApp from './components/ThemedApp';
import { UISettingsProvider } from './store/uiSettings';
import './index.css';

dayjs.locale('ru');

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
});

// Оформление зависит от персональных настроек, а те лежат в учётке —
// значит их надо читать хуком, а хук работает только внутри провайдеров.
// Поэтому ConfigProvider вынесен в ThemedApp, а здесь остались только
// поставщики данных.
createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <UISettingsProvider>
        <ThemedApp />
      </UISettingsProvider>
    </QueryClientProvider>
  </StrictMode>,
);
