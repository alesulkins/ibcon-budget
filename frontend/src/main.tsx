import React from 'react';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ConfigProvider } from 'antd';
import ruRU from 'antd/locale/ru_RU';
import dayjs from 'dayjs';
import 'dayjs/locale/ru';
import App from './App';
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

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <ConfigProvider
        locale={ruRU}
        theme={{
          // cssVar переводит antd на CSS-переменные: стили считаются один
          // раз, а не пересобираются рантайм-движком на каждую новую
          // комбинацию компонентов. hashed: false убирает хеш-классы —
          // меньше работы стилевому движку и чище инспектор.
          cssVar: { key: 'ibcon' },
          hashed: false,
          token: {
            colorPrimary: '#1a3a6b',
            // Совпадает с --ibcon-bg в index.css: иначе на краях страницы
            // виден стык двух почти одинаковых серых.
            colorBgLayout: '#f2f4f7',
            borderRadius: 8,
            fontSize: 14,
          },
        }}
      >
        <App />
      </ConfigProvider>
    </QueryClientProvider>
  </StrictMode>,
);
