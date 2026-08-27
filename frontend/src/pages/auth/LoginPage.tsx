import { useEffect, useState } from 'react';
import { Form, Input, Button, Card, Typography, Alert, Checkbox } from 'antd';
import { LockOutlined, MailOutlined } from '@ant-design/icons';
import { authApi } from '../../api';
import { setAuth, savedEmail } from '../../store/auth';
import { extractError } from '../../api/client';
import { BRAND, BRAND_LIGHT, PAGE_BG } from '../../theme';

const { Text } = Typography;

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
    // Экран входа вне каркаса приложения, поэтому прокручивается сам —
    // но только внутри себя: документ по-прежнему зафиксирован.
    <div style={{
      height: '100%',
      overflowY: 'auto',
      overscrollBehavior: 'none',
      background: PAGE_BG,
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      padding: 16,
    }}>
      {/* ibcon-panel возвращает карточке поверхность: в рабочей области
          плиты сняты, но здесь форма стоит одна на пустом фоне, и без
          собственной поверхности ей не на чем держаться. */}
      <Card
        className="ibcon-panel"
        style={{ width: 400, overflow: 'hidden' }}
        styles={{ body: { padding: 0 } }}
      >
        {/* Знак компании залит фирменным белым, поэтому стоит на
            фирменной плашке, а не на светлой карточке. */}
        <div style={{
          background: `linear-gradient(170deg, ${BRAND_LIGHT} 0%, ${BRAND} 100%)`,
          padding: '26px 24px 22px',
          textAlign: 'center',
        }}>
          <img
            src="/logo.svg"
            alt="IBCON"
            style={{ height: 30, width: 'auto', display: 'inline-block' }}
          />
          <div style={{
            marginTop: 10,
            color: 'rgba(253, 249, 248, 0.72)',
            fontSize: 12,
          }}>
            Система расчёта бюджетов проектов
          </div>
        </div>

        <div style={{ padding: 24 }}>
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
            >
              Войти
            </Button>
          </Form.Item>
        </Form>

        <Text type="secondary" style={{ fontSize: 12, display: 'block', lineHeight: 1.6 }}>
          Требования к паролю: не менее 10 символов, минимум 1 заглавная буква
          и 1 цифра. После 5 неуспешных попыток вход блокируется на 15 минут.
        </Text>
        </div>
      </Card>
    </div>
  );
}
