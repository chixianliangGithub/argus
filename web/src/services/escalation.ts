import request from '../utils/request';

export const getEscalations = async (params?: any) => {
  return request.get('/escalations', { params });
};

export const addEscalation = async (data: any) => {
  return request.post('/escalations', data);
};

export const updateEscalation = async (id: number, data: any) => {
  return request.put(`/escalations/${id}`, data);
};

export const deleteEscalation = async (id: number) => {
  return request.delete(`/escalations/${id}`);
};

