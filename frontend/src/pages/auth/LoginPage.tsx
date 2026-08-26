import { useEffect, useState } from 'react';
import { Form, Input, Button, Card, Typography, Alert, Checkbox } from 'antd';
import { LockOutlined, MailOutlined } from '@ant-design/icons';
import { authApi } from '../../api';
import { setAuth, savedEmail } from '../../store/auth';
import { extractError } from '../../api/client';
import { BRAND } from '../../theme';

const { Title, Text } = Typography;

interface LoginFormValues {
  email: string;
  password: string;
  remember: boolean;
}

/** Пояснение, почему пользователя вернуло на форму (?reason=…). */
function reasonNotice(reason: string | null): string {
  switch (reason) {
    case 'timeout':
      return 'Сессия завершена автоматически: 60 минут без активности. Войдите снова.';
    case 'disabled':
      return 'Учётная запись отключена. Обратитесь к главному экономисту.';
    default:
      return '';
  }
}

export default function LoginPage() {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [form] = Form.useForm<LoginFormValues>();

  const remembered = savedEmail();

  useEffect(() => {
    const reason = new URLSearchParams(window.location.search).get('reason');
    setNotice(reasonNotice(reason));
    // Убираем ?reason из адреса, чтобы подпись не всплыла при перезагрузке
    if (reason) {
      window.history.replaceState({}, '', '/login');
    }
  }, []);

  async function onFinish(values: LoginFormValues) {
    setError('');
    setNotice('');
    setLoading(true);
    try {
      const res = await authApi.login(values.email, values.password, values.remember);
      setAuth(
        res.token,
        {
          id: res.user_id,
          email: values.email,
          full_name: res.full_name,
          role: res.role,
          active: true,
          failed_attempts: 0,
          created_at: new Date().toISOString(),
        },
        values.remember,
      );
      window.location.replace('/projects');
    } catch (e) {
      setError(extractError(e));
    } finally {
      setLoading(false);
    }
  }

  return (
    <div style={{
      minHeight: '100vh',
      background: '#f0f2f5',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      padding: 16,
    }}>
      <Card style={{ width: 400, boxShadow: '0 4px 24px rgba(0,0,0,0.08)' }}>
        <div style={{ textAlign: 'center', marginBottom: 28 }}>
          <Title level={3} style={{ color: BRAND, margin: 0 }}>IBCON Бюджет</Title>
          <Text type="secondary">Система расчёта бюджетов проектов</Text>
        </div>

        {notice && (
          <Alert message={notice} type="warning" showIcon style={{ marginBottom: 16 }} />
        )}
        {error && (
          <Alert message={error} type="error" showIcon style={{ marginBottom: 16 }} />
        )}

        <Form
          form={form}
          layout="vertical"
          onFinish={onFinish}
          autoComplete="on"
          initialValues={{ email: remembered, remember: !!remembered }}
          requiredMark={false}
        >
          <Form.Item
            label="Email"
            name="email"
            rules={[
              { required: true, message: 'Введите email' },
              { type: 'email', message: 'Введите корректный email' },
            ]}
          >
            <Input
              autoComplete="email"
              size="large"
              prefix={<MailOutlined style={{ color: '#bfbfbf' }} />}
              placeholder="admin@ibcon.ru"
              autoFocus={!remembered}
            />
          </Form.Item>

          <Form.Item
            label="Пароль"
            name="password"
            rules={[{ required: true, message: 'Введите пароль' }]}
            style={{ marginBottom: 12 }}
          >
            <Input.Password
              autoComplete="current-password"
              size="large"
              prefix={<LockOutlined style={{ color: '#bfbfbf' }} />}
              autoFocus={!!remembered}
            />
          </Form.Item>

          <Form.Item name="remember" valuePropName="checked" style={{ marginBottom: 16 }}>
            <Checkbox>Запомнить меня</Checkbox>
          </Form.Item>

          <Form.Item style={{ marginBottom: 12 }}>
            <Button
              type="primary"
              htmlType="submit"
              size="large"
              block
              loading={loading}
              style={{ background: BRAND }}
            >
              Войти
            </Button>
          </Form.Item>
        </Form>

        <Text type="secondary" style={{ fontSize: 12, display: 'block', lineHeight: 1.6 }}>
          Требования к паролю: не менее 10 символов, минимум 1 заглавная буква
          и 1 цифра. После 5 неуспешных попыток вход блокируется на 15 минут.
        </Text>
      </Card>
    </div>
  );
}
