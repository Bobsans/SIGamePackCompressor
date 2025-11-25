export interface PackInfoSchema {
  type: "info";
  size: number;
  version: number;
  items_count: number;
}

export interface LogEntrySchema {
  type: "log";
  content: string;
}

export interface ErrorSchema {
  type: "error";
  error: string;
}

export interface OptimizeResultSchema {
  type: "result";
  url: string;
}

export type EntryType = PackInfoSchema | LogEntrySchema | ErrorSchema | OptimizeResultSchema;
