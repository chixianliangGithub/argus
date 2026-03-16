import request from '../utils/request';

export const getAlarms = async (params?: any) => {
  return request.get('/alarms', { params });
};

export const claimAlarm = async (id: number) => {
  return request.post(`/alarms/${id}/claim`);
};

export const unclaimAlarm = async (id: number) => {
  return request.post(`/alarms/${id}/unclaim`);
};

export const handleAlarm = async (id: number, data: { result: string; note?: string }) => {
  return request.post(`/alarms/${id}/handle`, data);
};

export const batchClaimAlarms = async (ids: number[]) => {
  return request.post(`/alarms/batch-claim`, { ids });
};

export const batchUnclaimAlarms = async (ids: number[]) => {
  return request.post(`/alarms/batch-unclaim`, { ids });
};

export const batchHandleAlarms = async (ids: number[], data: { result: string; note?: string }) => {
  return request.post(`/alarms/batch-handle`, { ids, ...data });
};

export const batchResolveAlarms = async (ids: number[]) => {
  return request.post(`/alarms/batch-resolve`, { ids });
};

export const batchSilenceAlarms = async (ids: number[], data: { duration_mins: number; comment?: string }) => {
  return request.post(`/alarms/batch-silence`, { ids, ...data });
};
