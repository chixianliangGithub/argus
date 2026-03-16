import request from '../utils/request';

export const queryData = async (data: { datasource_id: number; query: string; start?: number; end?: number; step?: number }) => {
  return request.post('/query', data);
};

