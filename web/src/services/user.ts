import request from '../utils/request';

export const getUsers = async (params?: any) => {
  return request.get('/users', { params });
};

export const addUser = async (data: any) => {
  return request.post('/users', data);
};

export const updateUser = async (id: number, data: any) => {
  return request.put(`/users/${id}`, data);
};

export const deleteUser = async (id: number) => {
  return request.delete(`/users/${id}`);
};

export const getUserInfo = async () => {
  return request.get('/user/info');
};
