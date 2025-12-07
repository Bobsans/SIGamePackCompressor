export interface PackInfoSchema {
  type: "info";
  size: number;
  version: number;
  items_count: number;
}

export interface OptimizeResultSchema {
  type: "result";
  url: string;
}

export interface LogEntrySchema {
  type: "log";
  data: LogEntryDataType;
}

export type LogEntryDataType = LogEntryUploading | LogEntryCompressed | LogEntryCompressError;

export interface LogEntryData {
  id?: string;
}

export interface LogEntryUploading extends LogEntryData {
  event: "uploading";
  percent: number;
}

export interface LogEntryCompressed extends LogEntryData  {
  event: "compressed";
  type: string;
  old_name: string;
  new_name: string;
  old_size: number;
  new_size: number;
}

export interface LogEntryCompressError extends LogEntryData  {
  event: "error";
  type?: string;
  name?: string;
  size?: number;
  error: string;
}


export type EntryType = PackInfoSchema | LogEntrySchema | OptimizeResultSchema;
