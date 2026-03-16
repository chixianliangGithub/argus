import request from '../utils/request';

export const getNotificationChannels = async (params?: any) => {
  return request.get('/notification-channels', { params });
};

export const addNotificationChannel = async (data: any) => {
  return request.post('/notification-channels', data);
};

export const updateNotificationChannel = async (id: number, data: any) => {
  return request.put(`/notification-channels/${id}`, data);
};

export const deleteNotificationChannel = async (id: number) => {
  return request.delete(`/notification-channels/${id}`);
};

export const testNotificationChannel = async (id: number, data: any) => {
  return request.post(`/notification-channels/${id}/test`, data);
};
