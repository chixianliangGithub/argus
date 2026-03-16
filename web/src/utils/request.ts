import axios from 'axios';
import { message } from 'antd';

const createClient = (timeout: number) =>
  axios.create({
    baseURL: '/api',
    timeout,
  });

const applyInterceptors = (client: any) => {
  client.interceptors.request.use(
    (config: any) => {
      const token = localStorage.getItem('token');
      if (token) {
        config.headers.Authorization = `Bearer ${token}`;
      }
      return config;
    },
    (error: any) => {
      return Promise.reject(error);
    }
  );

  client.interceptors.response.use(
    (response: any) => {
      return response.data;
    },
    (error: any) => {
      if (error.response) {
        const { status, data } = error.response;
        if (status === 401) {
          const token = localStorage.getItem('token');
          const reqUrl = String(error?.config?.url || '');
          const onLoginPage = window.location.pathname === '/login';
          const isAuthRequest =
            reqUrl.includes('/login') || reqUrl.includes('/register') || reqUrl.includes('/2fa/');
          if (!token || onLoginPage || isAuthRequest) {
            return Promise.reject(error);
          }
          localStorage.removeItem('token');
          window.location.href = '/login';
        } else {
          message.error(data.error || data.msg || 'Request failed');
        }
      } else {
        const msg = String(error?.message || '').toLowerCase();
        if (error?.code === 'ECONNABORTED' || msg.includes('timeout')) {
          message.error('Request timeout');
        } else {
          message.error('Network error');
        }
      }
      return Promise.reject(error);
    }
  );
};

const request = createClient(10000);
const longRequest = createClient(120000);

applyInterceptors(request);
applyInterceptors(longRequest);

export default request;
export { longRequest };
