import { useState } from 'react';
import { Alert, Button, Collapse, Input, Spin, Typography } from 'antd';
import { QuestionCircleOutlined, SendOutlined } from '@ant-design/icons';
import { useMutation, useQuery } from '@tanstack/react-query';
import { marketApi } from '../../../api';
import { extractError } from '../../../api/client';
import type { RentMarketEstimate } from '../../../types';

const { Text, Paragraph } = Typography;

/**
 * «Почему такая цена» — раскрывающаяся панель под результатом.
 *
 * Свёрнута по умолчанию: основное окно остаётся прежним, а пояснения
 * читают, только когда цифра вызвала вопрос.
 *
 * Внутри две разные вещи. Методика — постоянный текст с сервера: как
 * считается цена, чем медиана отличается от среднего, что такое
 * перцентиль и какие объявления отбрасываются. И вопрос ассистенту — он
 * отвечает по цифрам ЭТОГО расчёта; их платформа передаёт ему сама.
 */
export default function RentWhyPanel({ est }: { est: RentMarketEstimate }) {
  const [open, setOpen] = useState(false);
  const [question, setQuestion] = useState('');
  const [answer, setAnswer] = useState('');

  const { data: blocks } = useQuery({
    queryKey: ['market-methodology'],
    queryFn: marketApi.methodology,
    // Текст методики не меняется от расчёта к расчёту.
    staleTime: Infinity,
    enabled: open,
  });

  const ask = useMutation({
    mutationFn: () => marketApi.ask(question.trim(), est),
    onSuccess: setAnswer,
  });

  return (
    <div style={{ marginTop: 12 }}>
      <Button
        type="text"
        icon={<QuestionCircleOutlined />}
        onClick={() => setOpen(v => !v)}
        style={{ paddingLeft: 0 }}
      >
        Почему такая цена?
      </Button>

      {open && (
        <div className="ibcon-glass" style={{ padding: 14, marginTop: 8 }}>
          <Collapse
            ghost
            size="small"
            items={(blocks ?? []).map((b, i) => ({
              key: String(i),
              label: b.title,
              children: <Paragraph style={{ marginBottom: 0 }}>{b.text}</Paragraph>,
            }))}
          />

          <div style={{ marginTop: 12 }}>
            <Text type="secondary" style={{ fontSize: 12 }}>
              Спросите ассистента про этот расчёт — он видит его цифры.
            </Text>
            <div style={{ display: 'flex', gap: 8, marginTop: 6 }}>
              <Input
                placeholder="Например: почему верх рынка вдвое выше цены для бюджета?"
                value={question}
                onChange={(e) => setQuestion(e.target.value)}
                onPressEnter={() => { if (question.trim()) ask.mutate(); }}
                maxLength={500}
              />
              <Button
                type="primary"
                icon={<SendOutlined />}
                loading={ask.isPending}
                disabled={question.trim() === ''}
                onClick={() => ask.mutate()}
              >
                Спросить
              </Button>
            </div>
          </div>

          {ask.isPending && (
            <div style={{ marginTop: 10 }}><Spin size="small" /> </div>
          )}

          {ask.isError && (
            <Alert
              style={{ marginTop: 10 }}
              type="warning"
              showIcon
              message={extractError(ask.error)}
            />
          )}

          {answer && !ask.isPending && (
            <div className="ibcon-glass" style={{ padding: 12, marginTop: 10 }}>
              <Paragraph style={{ marginBottom: 0, whiteSpace: 'pre-wrap' }}>
                {answer}
              </Paragraph>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
