import { useEffect, useState } from 'react';
import {
  Card, Avatar, Typography, Button, Input, Space, message, Modal,
  Form, Row, Col, Descriptions, Upload, Alert, Popconfirm,
} from 'antd';
import {
  LockOutlined, UploadOutlined, SaveOutlined, DeleteOutlined,
} from '@ant-design/icons';
import type { UploadProps } from 'antd';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { profileApi } from '../../api';
import { ROLE_LABELS } from '../../types';
import type { Profile } from '../../types';
import { initials } from '../../utils/names';
import { extractError } from '../../api/client';
import { BRAND } from '../../theme';
import StatusTag from '../../components/StatusTag';

const { Text, Paragraph } = Typography;
const { TextArea } = Input;


/** Набор аватаров-стикеров: выбор не требует загрузки файла. */
const STICKERS = [
  '🙂', '🙄', '👸🏼', '👸🏻', '👩🏻‍💻', '👩🏼‍💻', '👨🏼‍💻', '👨🏼‍💼', 
  '🐇', '🦙', '🦢', '🐋', '🦕', '🌊',  '🪼','👾',
  '💡', '💵', '🏛️', '🏢', '🏗️', '⛰️', '📊', '🏙️',
];

/** Максимальный размер картинки аватара — 1 МБ до base64. */
const MAX_AVATAR_BYTES = 1024 * 1024;

export default function ProfilePage() {
  const qc = useQueryClient();
  const [notes, setNotes] = useState('');
  const [notesDirty, setNotesDirty] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const [passwordForm] = Form.useForm();

  const { data: profile, isLoading } = useQuery({
    queryKey: ['profile'],
    queryFn: profileApi.get,
  });

  useEffect(() => {
    if (profile) {
      setNotes(profile.notes ?? '');
      setNotesDirty(false);
    }
  }, [profile]);

  const updateMutation = useMutation({
    mutationFn: (data: { avatar?: string; notes?: string }) => profileApi.update(data),
    onSuccess: (p: Profile) => {
      qc.setQueryData(['profile'], p);
      setNotesDirty(false);
      message.success('Сохранено');
    },
    onError: (e) => message.error(extractError(e)),
  });

  const passwordMutation = useMutation({
    mutationFn: (v: { current: string; next: string }) =>
      profileApi.changePassword(v.current, v.next),
    onSuccess: () => {
      message.success('Пароль изменён');
      setShowPassword(false);
      passwordForm.resetFields();
    },
    onError: (e) => message.error(extractError(e)),
  });

  /** Загрузка картинки: читаем в data:-URL, на сервер уходит строкой. */
  const uploadProps: UploadProps = {
    accept: 'image/*',
    showUploadList: false,
    beforeUpload: (file) => {
      if (file.size > MAX_AVATAR_BYTES) {
        message.error('Файл больше 1 МБ — выберите изображение поменьше');
        return Upload.LIST_IGNORE;
      }
      const reader = new FileReader();
      reader.onload = () => updateMutation.mutate({ avatar: String(reader.result) });
      reader.onerror = () => message.error('Не удалось прочитать файл');
      reader.readAsDataURL(file);
      // Файл отправляем сами, штатная загрузка antd не нужна
      return Upload.LIST_IGNORE;
    },
  };

  function confirmPasswordChange(vals: { current: string; next: string }) {
    Modal.confirm({
      title: 'Изменить пароль?',
      content: 'После смены пароля вход по старому станет невозможен.',
      okText: 'Да, изменить',
      cancelText: 'Отмена',
      onOk: () => passwordMutation.mutateAsync(vals),
    });
  }

  if (isLoading || !profile) return null;

  const avatar = profile.avatar ?? '';
  const isImage = avatar.startsWith('data:');

  return (
    // Заголовок «Личный кабинет» живёт в шапке (AppLayout), здесь его нет.
    <div style={{ maxWidth: 980 }}>
      <Row gutter={16} align="stretch">
        {/* ── Аватар и реквизиты ─────────────────────────────────── */}
        <Col xs={24} md={10}>
          <Card size="small" style={{ height: '100%' }}>
            <div style={{ textAlign: 'center', marginBottom: 16 }}>
              <Avatar
                size={96}
                src={isImage ? avatar : undefined}
                style={{ background: isImage ? undefined : BRAND, fontSize: 40 }}
              >
                {!isImage && (avatar || initials(profile.full_name))}
              </Avatar>

              <div style={{ marginTop: 12 }}>
                <Space>
                  <Upload {...uploadProps}>
                    <Button size="small" icon={<UploadOutlined />}>Загрузить фото</Button>
                  </Upload>
                  {avatar && (
                    <Popconfirm
                      title="Убрать аватар?"
                      okText="Да"
                      cancelText="Нет"
                      okButtonProps={{ danger: true }}
                      onConfirm={() => updateMutation.mutate({ avatar: '' })}
                    >
                      <Button size="small" icon={<DeleteOutlined />}>Убрать</Button>
                    </Popconfirm>
                  )}
                </Space>
              </div>
            </div>

            <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 8 }}>
              Или выберите стикер:
            </Text>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, marginBottom: 16 }}>
              {STICKERS.map((s) => (
                <Button
                  key={s}
                  size="small"
                  type={avatar === s ? 'primary' : 'default'}
                  style={{
                    fontSize: 18,
                    width: 38,
                    height: 38,
                    padding: 0,
                    background: avatar === s ? BRAND : undefined,
                  }}
                  onClick={() => updateMutation.mutate({ avatar: s })}
                >
                  {s}
                </Button>
              ))}
            </div>

            <Descriptions column={1} size="small" bordered>
              <Descriptions.Item label="ФИО">
                {profile.full_name}
              </Descriptions.Item>
              <Descriptions.Item label="Email">{profile.email}</Descriptions.Item>
              <Descriptions.Item label="Роль">
                <StatusTag color="blue">{ROLE_LABELS[profile.role] ?? profile.role}</StatusTag>
              </Descriptions.Item>
            </Descriptions>
            <Text type="secondary" style={{ fontSize: 12, display: 'block', marginTop: 8 }}>
              ФИО и роль меняет главный экономист в разделе «Пользователи».
            </Text>

            <Button
              icon={<LockOutlined />}
              style={{ marginTop: 12, width: '100%' }}
              onClick={() => setShowPassword(true)}
            >
              Изменить пароль
            </Button>
          </Card>
        </Col>

        {/* ── Рабочие заметки ────────────────────────────────────── */}
        <Col xs={24} md={14}>
          <Card
            size="small"
            title="Рабочие заметки и напоминания"
            style={{ height: '100%'}}
            extra={(
              <Button
                type="primary"
                size="small"
                icon={<SaveOutlined />}
                
                disabled={!notesDirty}
                loading={updateMutation.isPending}
                onClick={() => updateMutation.mutate({ notes })}
              >
                Сохранить
              </Button>
            )}
          >
            <Paragraph type="secondary" style={{ fontSize: 12 }}>
              Заметки видны только вам и хранятся в вашей учётной записи.
            </Paragraph>
            <TextArea
              value={notes}
              onChange={(e) => { setNotes(e.target.value); setNotesDirty(true); }}
              rows={18}
              placeholder="Например: пересчитать бюджет по проекту №12 после уточнения ТКП…"
            />
          </Card>
        </Col>
      </Row>

      {/* ── Смена пароля ─────────────────────────────────────────── */}
      <Modal
        title="Изменение пароля"
        open={showPassword}
        onCancel={() => { setShowPassword(false); passwordForm.resetFields(); }}
        onOk={() => passwordForm.submit()}
        confirmLoading={passwordMutation.isPending}
        okText="Изменить"
        cancelText="Отмена"
        destroyOnClose
      >
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message="Требования к паролю"
          description="Не менее 10 символов, минимум 1 заглавная буква и 1 цифра."
        />
        <Form form={passwordForm} layout="vertical" onFinish={confirmPasswordChange}>
          <Form.Item
            name="current"
            label="Текущий пароль"
            rules={[{ required: true, message: 'Введите текущий пароль' }]}
          >
            <Input.Password autoComplete="current-password" />
          </Form.Item>
          <Form.Item
            name="next"
            label="Новый пароль"
            rules={[
              { required: true, message: 'Введите новый пароль' },
              // Дублируем правила ТЗ на клиенте, чтобы ошибка была видна
              // сразу; сервер проверяет их независимо.
              {
                validator: (_, v: string) => {
                  if (!v) return Promise.resolve();
                  if ([...v].length < 10) {
                    return Promise.reject(new Error('Не менее 10 символов'));
                  }
                  if (!/\p{Lu}/u.test(v)) {
                    return Promise.reject(new Error('Нужна минимум 1 заглавная буква'));
                  }
                  if (!/\p{Nd}/u.test(v)) {
                    return Promise.reject(new Error('Нужна минимум 1 цифра'));
                  }
                  return Promise.resolve();
                },
              },
            ]}
          >
            <Input.Password autoComplete="new-password" />
          </Form.Item>
          <Form.Item
            name="repeat"
            label="Повторите новый пароль"
            dependencies={['next']}
            rules={[
              { required: true, message: 'Повторите новый пароль' },
              ({ getFieldValue }) => ({
                validator: (_, v: string) =>
                  !v || getFieldValue('next') === v
                    ? Promise.resolve()
                    : Promise.reject(new Error('Пароли не совпадают')),
              }),
            ]}
          >
            <Input.Password autoComplete="new-password" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
