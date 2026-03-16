import request, { longRequest } from '../utils/request';

export const createIncidentAIInsight = async (incidentId: number, data: any) => {
  return longRequest.post(`/incidents/${incidentId}/ai-insights`, data);
};

export const getIncidentAIInsights = async (incidentId: number, params?: any) => {
  return request.get(`/incidents/${incidentId}/ai-insights`, { params });
};
