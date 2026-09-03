import { Card, Col, Row } from 'antd';
import InterfaceSettings from '../profile/InterfaceSettings';

// Настройки интерфейса — свой раздел, а не подвал личного кабинета: в них
// заходят отдельно от работы с профилем, и в сайдбаре они стоят наравне с
// остальными разделами.
export default function SettingsPage() {
  return (
    <Row gutter={16}>
      <Col xs={24} md={12} lg={9}>
        <Card size="small" title="Интерфейс">
          <InterfaceSettings />
        </Card>
      </Col>
    </Row>
  );
}
