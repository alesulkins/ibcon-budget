import React, { useState } from 'react';
import { Form, Input, Button, Card, Typography, Alert } from 'antd';
import { authApi } from '../../api';
import { setAuth } from '../../store/auth';
import { extractError } from '../../api/client';

const { Title } = Typography;

export default function LoginPage() {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  async function onFinish(values: { email: string; password: string }) {
    setError('');
    setLoading(true);
    try {
      const res = await authApi.login(values.email, values.password);
      setAuth(res.token, {
        id: res.user_id,
        email: values.email,
        full_name: res.full_name,
        role: res.role,
        active: true,
        failed_attempts: 0,
        created_at: new Date().toISOString(),
      });
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
    }}>
      <Card style={{ width: 400, boxShadow: '0 4px 24px rgba(0,0,0,0.08)' }}>
        <div style={{ textAlign: 'center', marginBottom: 32 }}>
          <Title level={3} style={{ color: '#1a3a6b', margin: 0 }}>IBCON Бюджет</Title>
          <Typography.Text type="secondary">Система расчёта бюджетов проектов</Typography.Text>
        </div>

        {error && <Alert message={error} type="error" showIcon style={{ marginBottom: 16 }} />}

        <Form layout="vertical" onFinish={onFinish} autoComplete="on">
          <Form.Item
            label="Email"
            name="email"
            rules={[{ required: true, message: 'Введите email' }]}
          >
            <Input autoComplete="email" size="large" placeholder="admin@ibcon.ru" />
          </Form.Item>

          <Form.Item
            label="Пароль"
            name="password"
            rules={[{ required: true, message: 'Введите пароль' }]}
          >
            <Input.Password autoComplete="current-password" size="large" />
          </Form.Item>

          <Form.Item style={{ marginBottom: 0 }}>
            <Button
              type="primary"
              htmlType="submit"
              size="large"
              block
              loading={loading}
              style={{ background: '#1a3a6b' }}
            >
              Войти
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
}
