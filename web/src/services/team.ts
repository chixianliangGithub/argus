import request from '../utils/request';

export const getTeams = async (params?: any) => {
  return request.get('/teams', { params });
};

export const addTeam = async (data: any) => {
  return request.post('/teams', data);
};

export const updateTeam = async (id: number, data: any) => {
  return request.put(`/teams/${id}`, data);
};

export const deleteTeam = async (id: number) => {
  return request.delete(`/teams/${id}`);
};

export const addTeamMember = async (id: number, userId: number) => {
  return request.post(`/teams/${id}/members`, { user_id: userId });
};

export const removeTeamMember = async (id: number, userId: number) => {
  return request.delete(`/teams/${id}/members/${userId}`);
};
