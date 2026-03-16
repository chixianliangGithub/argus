import request from '../utils/request';

export const getExecutions = async (params?: any) => {
  return request.get('/runbooks/executions', { params });
};

export const getExecution = async (id: number) => {
  return request.get(`/runbooks/executions/${id}`);
};

export const approveExecution = async (id: number, approved: boolean, feedback: string) => {
  return request.post(`/runbooks/executions/${id}/approve`, { approved, feedback });
};
