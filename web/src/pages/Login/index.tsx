import React, { useState } from 'react';
import { LoginForm, ProFormText } from '@ant-design/pro-components';
import { UserOutlined, LockOutlined } from '@ant-design/icons';
import { message, Tabs, Typography } from 'antd';
import { useNavigate } from 'react-router-dom';
import { confirmTwoFASetup, login } from '../../services/auth';

const Login: React.FC = () => {
  const navigate = useNavigate();
  const [need2FA, setNeed2FA] = useState(false);
  const [setupToken, setSetupToken] = useState('');
  const [setupQR, setSetupQR] = useState('');
  const [setupSecret, setSetupSecret] = useState('');
  const [setupOtpauth, setSetupOtpauth] = useState('');

  const handleSubmit = async (values: any) => {
    try {
      if (setupToken) {
        const code = String(values?.totp_code || '').trim();
        const res: any = await confirmTwoFASetup(setupToken, code);
        if (res?.token) {
          message.success('绑定成功，登录成功');
          localStorage.setItem('token', res.token);
          navigate('/dashboard');
          return true;
        }
        message.error('登录失败');
        return false;
      }

      const payload = { ...values };
      if (!need2FA) delete payload.totp_code;
      const res: any = await login(payload);
      if (res.token) {
        message.success('登录成功');
        localStorage.setItem('token', res.token);
        navigate('/dashboard');
        return true;
      }
      message.error('登录失败');
      return false;
    } catch (error: any) {
      const err = error?.response?.data?.error || error?.message || '登录失败，请检查网络连接';
      if (String(err) === '2fa_required') {
        setNeed2FA(true);
        setSetupToken('');
        setSetupQR('');
        setSetupSecret('');
        setSetupOtpauth('');
        message.warning('该账号已开启二次验证，请输入谷歌验证码');
        return false;
      }
      if (String(err) === '2fa_setup_required') {
        const data = error?.response?.data || {};
        setNeed2FA(true);
        setSetupToken(String(data?.setup_token || '').trim());
        setSetupQR(String(data?.qr_png_base64 || '').trim());
        setSetupSecret(String(data?.secret || '').trim());
        setSetupOtpauth(String(data?.otpauth_url || '').trim());
        message.warning('系统已开启全局二次验证，请先绑定谷歌验证器');
        return false;
      }
      if (String(err) === 'twofa_key_not_configured') {
        message.error('2FA 密钥未配置，请先在服务端配置 two_fa_secret_key');
        return false;
      }
      if (String(err) === '2fa_secret_invalid') {
        setNeed2FA(true);
        setSetupToken('');
        setSetupQR('');
        setSetupSecret('');
        setSetupOtpauth('');
        message.error('2FA 绑定信息已失效，请重新登录触发绑定');
        return false;
      }
      if (String(err) === 'invalid_2fa_code') {
        setNeed2FA(true);
        message.error('谷歌验证码错误');
        return false;
      }
      message.error(String(err));
      return false;
    }
  };

  return (
    <div style={{ height: '100vh', display: 'flex', justifyContent: 'center', alignItems: 'center', background: '#141414' }}>
      <LoginForm
        title="Argus Monitor"
        subTitle="企业级监控系统"
        onFinish={handleSubmit}
      >
        <Tabs
          centered
          items={[
            {
              key: 'account',
              label: '账号密码登录',
            },
          ]}
        />
        <ProFormText
          name="username"
          fieldProps={{
            size: 'large',
            prefix: <UserOutlined className={'prefixIcon'} />,
            disabled: !!setupToken,
          }}
          rules={[
            {
              required: true,
              message: '请输入用户名!',
            },
          ]}
        />
        <ProFormText.Password
          name="password"
          fieldProps={{
            size: 'large',
            prefix: <LockOutlined className={'prefixIcon'} />,
            disabled: !!setupToken,
          }}
          rules={[
            {
              required: true,
              message: '请输入密码！',
            },
          ]}
        />
        {setupToken && setupQR ? (
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 8, marginBottom: 8 }}>
            <img
              alt="2fa-qrcode"
              style={{ width: 280, height: 280, borderRadius: 8, background: '#fff', padding: 12 }}
              src={`data:image/png;base64,${setupQR}`}
            />
            {setupSecret ? <div style={{ fontFamily: 'monospace' }}>{setupSecret}</div> : null}
            {setupOtpauth ? (
              <Typography.Text style={{ maxWidth: 520 }} ellipsis={{ tooltip: setupOtpauth }}>
                {setupOtpauth}
              </Typography.Text>
            ) : null}
          </div>
        ) : null}
        {need2FA ? (
          <ProFormText
            name="totp_code"
            fieldProps={{
              size: 'large',
              prefix: <LockOutlined className={'prefixIcon'} />,
              maxLength: 6,
            }}
            rules={[
              {
                required: true,
                message: '请输入谷歌验证码！',
              },
            ]}
            placeholder="谷歌验证码(6位)"
          />
        ) : null}
      </LoginForm>
    </div>
  );
};

export default Login;
