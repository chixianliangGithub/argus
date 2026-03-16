import request from '../utils/request';

export const getAuditLogs = async (params?: any) => {
  return request.get('/audit-logs', { params });
};
