import request from '../utils/request';

export const getDataNames = async (params?: any) => {
  return request.get('/datanames', { params });
};

export const addDataName = async (data: any) => {
  return request.post('/datanames', data);
};

export const updateDataName = async (id: number, data: any) => {
  return request.put(`/datanames/${id}`, data);
};

export const deleteDataName = async (id: number) => {
  return request.delete(`/datanames/${id}`);
};
