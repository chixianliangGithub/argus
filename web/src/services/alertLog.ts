import request from '../utils/request';

export const getAlertLogs = async (params?: any) => {
  return request.get('/alert-logs', { params });
};

