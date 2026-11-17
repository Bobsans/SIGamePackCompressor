export interface CompressionOptions {
  output_version: "5" | "source";
  images: {
    enabled: boolean;
    max_width: number;
    max_height: number;
    quality: number;
    format: "webp" | "source";
    lossless: boolean;
    animation: "preserve" | "webp";
  };
  audio: { enabled: boolean; format: "mp3" | "opus"; bitrate_kbps: number };
  video: {
    enabled: boolean;
    max_width: number;
    max_height: number;
    crf: number;
    preset: string;
    audio_bitrate_kbps: number;
  };
}

export interface Capabilities {
  defaults: CompressionOptions;
  input_versions: string[];
  output_versions: string[];
  image_formats: string[];
  tools: Record<string, boolean>;
  max_upload_bytes: number;
}

export interface MediaFile {
  name: string;
  type: string;
  size: number;
  extension: string;
  references: number;
}

export interface MediaDetails {
  width?: number;
  height?: number;
  frames?: number;
  duration_seconds?: number;
  codec?: string;
  format?: string;
}

export interface UploadedPack {
  id: string;
  filename: string;
  created_at: string;
  analysis: {
    name: string;
    version: string;
    size: number;
    expanded_size: number;
    rounds: number;
    themes: number;
    questions: number;
    media: MediaFile[];
    warnings: string[];
    output_versions: string[];
    unused_files?: number;
    unused_size?: number;
  };
  media_details: Record<string, MediaDetails>;
}

export type JobStatus = "queued" | "running" | "completed" | "failed" | "cancelled";

export interface FileLogEntry {
  sequence: number;
  name: string;
  type: string;
  status: "processing" | "compressed" | "unchanged" | "interrupted";
  old_size: number;
  new_size?: number;
  warning?: string;
  started_at: string;
}

export interface CompressionJob {
  id: string;
  upload_id: string;
  status: JobStatus;
  options: CompressionOptions;
  progress: number;
  total: number;
  created_at: string;
  updated_at: string;
  error?: string;
  warnings: string[];
  download_url?: string;
  original_size: number;
  result_size?: number;
  log?: FileLogEntry[];
  removed_files?: number;
  removed_bytes?: number;
}

export type JobUpdate =
  | { type: "snapshot"; state: CompressionJob }
  | { type: "status"; state: Omit<CompressionJob, "log"> }
  | { type: "file"; entry: FileLogEntry };
