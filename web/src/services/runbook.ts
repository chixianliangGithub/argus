import request from '../utils/request';

export const matchRunbooks = async (data: { query: string }) => {
  return request.post('/runbooks/match', data);
};

export const executeRunbook = async (id: number, data: any) => {
  return request.post(`/runbooks/${id}/execute`, data);
};

