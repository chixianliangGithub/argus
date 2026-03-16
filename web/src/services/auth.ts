import request from '../utils/request';

export const login = async (params: any) => {
  return request.post('/login', params);
};

export const register = async (params: any) => {
  return request.post('/register', params);
};

export const confirmTwoFASetup = async (setup_token: string, code: string) => {
  return request.post('/2fa/setup/confirm', { setup_token, code });
};
