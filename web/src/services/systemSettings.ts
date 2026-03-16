import request from '../utils/request';

export const getGlobalTwoFASetting = async () => {
  return request.get('/system-settings/2fa');
};

export const setGlobalTwoFASetting = async (enabled: boolean) => {
  return request.put('/system-settings/2fa', { enabled });
};

