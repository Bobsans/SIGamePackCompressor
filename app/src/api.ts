import axios, { type AxiosRequestConfig } from "axios";

export const API_URL = "<![API_SERVER_URL]!>";

const makeUrl = (path: string) => `${API_URL.replace(/\/$/g, "")}/${path.replace(/^\//g, "")}`;

export const post = (url: string, data: any, config: AxiosRequestConfig = {}) => axios.post(makeUrl(url), data, config);

export const compress = (file: File, config: AxiosRequestConfig = {}) => {
  const form = new FormData();
  form.append("file", file);
  return post("compress", form, config);
};


export const wsListen = (token: string, action: (data: any) => any) => {
  const ws = new WebSocket(makeUrl(`ws?token=${token}`).replace(/^http/, "ws"));
  ws.addEventListener("message", (e) => action(JSON.parse(e.data)));
};
