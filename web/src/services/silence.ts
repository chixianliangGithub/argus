import request from '../utils/request';

export const getSilences = async (params?: any) => {
  return request.get('/silences', { params });
};

export const addSilence = async (data: any) => {
  return request.post('/silences', data);
};

export const updateSilence = async (id: number, data: any) => {
  return request.put(`/silences/${id}`, data);
};

export const deleteSilence = async (id: number) => {
  return request.delete(`/silences/${id}`);
};
