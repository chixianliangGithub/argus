import request from '../utils/request';

export const getTemplates = async (params?: any) => {
  return request.get('/templates', { params });
};

export const addTemplate = async (data: any) => {
  return request.post('/templates', data);
};

export const updateTemplate = async (id: number, data: any) => {
  return request.put(`/templates/${id}`, data);
};

export const deleteTemplate = async (id: number) => {
  return request.delete(`/templates/${id}`);
};
