import React, { useEffect, useState } from 'react';
import { Card, Space, Typography } from 'antd';
import { getTwoFAStatus } from '../../services/twofa';

const { Text } = Typography;

const Security: React.FC = () => {
  const [loading, setLoading] = useState(false);
  const [enabled, setEnabled] = useState(false);
  const [globalEnabled, setGlobalEnabled] = useState(false);

  const reload = async () => {
    setLoading(true);
    try {
      const res: any = await getTwoFAStatus();
      setEnabled(!!res?.enabled);
      setGlobalEnabled(!!res?.global_enabled);
    } catch (e) {
      setEnabled(false);
      setGlobalEnabled(false);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    reload();
  }, []);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      <Card title="安全设置 - 二次验证(2FA)" loading={loading}>
        <Space direction="vertical" style={{ width: '100%' }} size={8}>
          <div>
            全局2FA：{globalEnabled ? <Text type="success">已开启</Text> : <Text type="secondary">未开启</Text>}
          </div>
          <div>
            状态：{enabled ? <Text type="success">已启用</Text> : <Text type="warning">未启用</Text>}
          </div>
          {!globalEnabled ? (
            <Text type="secondary">全局2FA未开启，无需绑定；请在系统管理中开启后在登录页完成绑定。</Text>
          ) : (
            <Text type="secondary">全局2FA已开启：未绑定用户会在登录时要求先绑定，绑定确认后才能进入系统。</Text>
          )}
        </Space>
      </Card>
    </div>
  );
};

export default Security;
