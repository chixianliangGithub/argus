import request from '../utils/request';

export const getAlertEvalRecords = async (params?: any) => {
  return request.get('/alert-eval-records', { params });
};

