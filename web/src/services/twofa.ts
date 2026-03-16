import request from '../utils/request';

export const getTwoFAStatus = async () => {
  return request.get('/2fa/status');
};

export const setupTwoFA = async () => {
  return request.post('/2fa/setup');
};

export const enableTwoFA = async (code: string) => {
  return request.post('/2fa/enable', { code });
};

export const disableTwoFA = async (password: string, code?: string) => {
  return request.post('/2fa/disable', { password, code });
};

