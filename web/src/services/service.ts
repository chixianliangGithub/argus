import request from '../utils/request';

export const getServicesOverview = async (params?: any) => {
  return request.get('/services/overview', { params });
};

export const getServiceSummary = async (service: string, params?: any) => {
  return request.get(`/services/${encodeURIComponent(service)}/summary`, { params });
};

