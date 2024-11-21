import api, { setToken } from './api';

const setAuthToken = (token: string | null) => {
  setToken(token);
};

export default setAuthToken; 