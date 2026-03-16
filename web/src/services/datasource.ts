import request from '../utils/request';

export const getDataSources = async (params?: any) => {
  return request.get('/datasources', { params });
};

export const addDataSource = async (data: any) => {
  return request.post('/datasources', data);
};

export const updateDataSource = async (id: number, data: any) => {
  return request.put(`/datasources/${id}`, data);
};

export const deleteDataSource = async (id: number) => {
  return request.delete(`/datasources/${id}`);
};
