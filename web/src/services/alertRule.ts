import request from '../utils/request';

export const getAlertRules = async (params?: any) => {
  return request.get('/alert-rules', { params });
};

export const getAlertRule = async (id: number) => {
  return request.get(`/alert-rules/${id}`);
};

export const addAlertRule = async (data: any) => {
  return request.post('/alert-rules', data);
};

export const updateAlertRule = async (id: number, data: any) => {
  return request.put(`/alert-rules/${id}`, data);
};

export const deleteAlertRule = async (id: number) => {
  return request.delete(`/alert-rules/${id}`);
};

export const previewAlertRule = async (data: any) => {
  return request.post('/alert-rules/preview', data);
};

export const triggerAlertRule = async (id: number, force = false) => {
  return request.post(`/alert-rules/${id}/trigger`, { force });
};

export const batchUpdateAlertRulesEnabled = async (ids: number[], is_enabled: boolean) => {
  return request.put('/alert-rules/batch-enable', { ids, is_enabled });
};

export const getAlertRuleRuntime = async (id: number) => {
  return request.get(`/alert-rules/${id}/runtime`);
};
