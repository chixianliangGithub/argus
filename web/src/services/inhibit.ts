import request from '../utils/request';

export const getInhibitRules = async (params?: any) => {
  return request.get('/inhibit-rules', { params });
};

export const addInhibitRule = async (data: any) => {
  return request.post('/inhibit-rules', data);
};

export const updateInhibitRule = async (id: number, data: any) => {
  return request.put(`/inhibit-rules/${id}`, data);
};

export const deleteInhibitRule = async (id: number) => {
  return request.delete(`/inhibit-rules/${id}`);
};

