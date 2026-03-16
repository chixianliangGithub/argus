import request from '../utils/request';

export const getIncidents = async (params?: any) => {
  return request.get('/incidents', { params });
};

export const getIncident = async (id: number) => {
  return request.get(`/incidents/${id}`);
};

export const ackIncident = async (id: number, note?: string) => {
  return request.post(`/incidents/${id}/ack`, { note: note || '' });
};

export const resolveIncident = async (id: number, note?: string) => {
  return request.post(`/incidents/${id}/resolve`, { note: note || '' });
};

export const mergeIncident = async (id: number, sourceIds: number[]) => {
  return request.post(`/incidents/${id}/merge`, { source_ids: sourceIds });
};

export const commentIncident = async (id: number, note: string) => {
  return request.post(`/incidents/${id}/comment`, { note });
};

export const updateIncident = async (id: number, data: any) => {
  return request.put(`/incidents/${id}`, data);
};

export const exportIncidentMarkdown = async (id: number) => {
  return request.get(`/incidents/${id}/export`, { responseType: 'blob' as any });
};
