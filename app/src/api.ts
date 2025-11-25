import axios, { type AxiosRequestConfig } from "axios";

export const API_URL = "<![API_SERVER_URL]!>";

export const post = (url: string, data: any, config: AxiosRequestConfig = {}) => axios.post(`${API_URL}/${url.replace(/^\//g, "")}`, data, config);

export const compress = (file: File, config: AxiosRequestConfig = {}) => {
  const form = new FormData();
  form.append("file", file);
  return post("compress", form, config);
};


export const wsListen = (url: string, token: string, action: (data: any) => any) => {
  const ws = new WebSocket(`${API_URL.replace(/^http/, "ws")}/${url}?token=${token}`);
  ws.addEventListener("message", (e) => action(JSON.parse(e.data)));
};
