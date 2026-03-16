import { longRequest } from '../utils/request';

export const aiInsight = async (data: { datasource_id: number; message: string }) => {
  return longRequest.post('/ai/insight', data);
};

export const aiTextToQuery = async (data: { datasource_type: string; query_text: string }) => {
  return longRequest.post('/ai/text-to-query', data);
};
