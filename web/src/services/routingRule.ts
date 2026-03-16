import request from '../utils/request';

export const getRoutingRules = async (params?: any) => {
  return request.get('/routing-rules', { params });
};

export const addRoutingRule = async (data: any) => {
  return request.post('/routing-rules', data);
};

export const updateRoutingRule = async (id: number, data: any) => {
  return request.put(`/routing-rules/${id}`, data);
};

export const deleteRoutingRule = async (id: number) => {
  return request.delete(`/routing-rules/${id}`);
};

export const previewRoutingRule = async (data: { alarm_id: number }) => {
  return request.post('/routing-rules/preview', data);
};

export const getRoutingRuleMatcherOptions = async (params?: any) => {
  return request.get('/routing-rules/matcher-options', { params });
};
