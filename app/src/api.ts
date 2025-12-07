import axios, { type AxiosRequestConfig } from "axios";
import type { EntryType } from "@/types";
import { urljoin } from "@/utils";

export const API_URL = "<![API_SERVER_URL]!>";

const makeUrl = (path: string) => urljoin(API_URL, path);

export const post = (url: string, data: any, config: AxiosRequestConfig = {}) => axios.post(makeUrl(url), data, config);

export const compress = (file: File, config: AxiosRequestConfig = {}) => {
  const form = new FormData();
  form.append("file", file);
  return post("compress", form, config);
};


export const wsListen = (token: string, action: (data: EntryType) => any) => {
  const ws = new WebSocket(makeUrl(`ws?token=${token}`).replace(/^http/, "ws"));
  ws.addEventListener("message", (e) => action(JSON.parse(e.data)));
};
