import { computed, onBeforeUnmount, ref, watch } from "vue";
import { API_URL, cancelJob, errorMessage, getCapabilities, getJob, getUpload, openJobStream, startJob, uploadPack } from "@/api";
import type { Capabilities, CompressionJob, CompressionOptions, JobUpdate, UploadedPack } from "@/types";

const isActive = (job: CompressionJob | null) => job?.status === "queued" || job?.status === "running";
const cloneOptions = (options: CompressionOptions): CompressionOptions => JSON.parse(JSON.stringify(options));

export function useCompression() {
  const capabilities = ref<Capabilities | null>(null);
  const upload = ref<UploadedPack | null>(null);
  const job = ref<CompressionJob | null>(null);
  const options = ref<CompressionOptions | null>(null);
  const initializing = ref(true);
  const uploading = ref(false);
  const starting = ref(false);
  const cancelling = ref(false);
  const uploadProgress = ref(0);
  const filename = ref("");
  const error = ref("");
  const connectionError = ref("");
  const sessionKey = `si-compressor:${API_URL}:session`;
  let epoch = 0;
  let uploadController: AbortController | null = null;
  let socket: WebSocket | null = null;
  let streamGeneration = 0;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let failures = 0;

  const stage = computed(() => {
    if (isActive(job.value)) return "processing";
    if (job.value) return "result";
    if (upload.value) return "settings";
    return "upload";
  });

  function persist() {
    try {
      if (!upload.value) sessionStorage.removeItem(sessionKey);
      else sessionStorage.setItem(sessionKey, JSON.stringify({ uploadId: upload.value.id, jobId: job.value?.id, options: options.value }));
    } catch { /* Storage is optional; processing works without it. */ }
  }

  watch(options, () => { if (!initializing.value) persist(); }, { deep: true });

  function stopUpdates() {
    clearTimeout(timer);
    streamGeneration++;
    if (socket) {
      socket.onclose = null;
      socket.onmessage = null;
      socket.close();
      socket = null;
    }
  }

  function connectUpdates() {
    if (!job.value || !isActive(job.value)) return;
    stopUpdates();
    const currentEpoch = epoch;
    const generation = streamGeneration;
    let next: WebSocket;
    try { next = openJobStream(job.value.id); }
    catch (reason) { connectionError.value = errorMessage(reason); scheduleReconnect(); return; }
    socket = next;
    const current = () => currentEpoch === epoch && generation === streamGeneration;
    next.onmessage = event => {
      if (!current() || !job.value) return;
      try {
        const message = JSON.parse(event.data) as JobUpdate;
        if (message.type === "snapshot") {
          job.value = message.state;
        } else if (message.type === "status") {
          Object.assign(job.value, message.state);
        } else if (message.type === "file") {
          const log = job.value.log ?? (job.value.log = []);
          const index = log.findIndex(entry => entry.sequence === message.entry.sequence);
          if (index >= 0) log[index] = message.entry;
          else log.push(message.entry);
        } else return;
        failures = 0;
        connectionError.value = "";
        if (message.type !== "file") persist();
        if (!isActive(job.value)) stopUpdates();
      } catch {
        connectionError.value = "Не удалось прочитать обновление — переподключаемся";
        next.close();
      }
    };
    next.onclose = () => {
      if (!current() || !isActive(job.value)) return;
      connectionError.value = "Связь с сервером прервалась — восстанавливаем журнал и статус задания";
      scheduleReconnect();
    };
    next.onerror = () => { /* onclose owns reconnection, avoiding duplicate retries. */ };
  }

  function scheduleReconnect() {
    clearTimeout(timer);
    failures++;
    timer = setTimeout(connectUpdates, Math.min(10_000, 1000 * 2 ** Math.min(failures - 1, 4)));
  }

  function retryStatus() { stopUpdates(); connectUpdates(); }

  function defaultsForPack() {
    if (!capabilities.value || !upload.value) return;
    const next = cloneOptions(capabilities.value.defaults);
    const media = upload.value.analysis.media;
    const tools = capabilities.value.tools;
    next.images.enabled = next.images.enabled && media.some(m => m.type === "image") && !!tools.vips && !!tools.vipsheader;
    next.audio.enabled = next.audio.enabled && media.some(m => m.type === "audio") && !!tools.ffmpeg && !!tools.ffprobe;
    next.video.enabled = next.video.enabled && media.some(m => m.type === "video") && !!tools.ffmpeg && !!tools.ffprobe;
    options.value = next;
  }

  async function initialize() {
    const currentEpoch = ++epoch;
    initializing.value = true;
    error.value = "";
    try {
      const config = await getCapabilities();
      if (currentEpoch !== epoch) return;
      capabilities.value = config;
      let saved: { uploadId?: string; jobId?: string; options?: CompressionOptions } | null = null;
      try { saved = JSON.parse(sessionStorage.getItem(sessionKey) ?? "null"); } catch { /* Ignore invalid saved state. */ }
      if (saved?.uploadId && /^[a-f0-9]{48}$/.test(saved.uploadId)) {
        try {
          const restored = await getUpload(saved.uploadId);
          if (currentEpoch !== epoch) return;
          upload.value = restored;
          defaultsForPack();
          if (saved.options?.images && saved.options?.audio && saved.options?.video) options.value = cloneOptions(saved.options);
          if (saved.jobId && /^[a-f0-9]{48}$/.test(saved.jobId)) {
            const restoredJob = await getJob(saved.jobId);
            if (currentEpoch !== epoch) return;
            job.value = restoredJob;
            if (isActive(job.value)) connectUpdates();
          }
        } catch {
          error.value = "Не удалось восстановить предыдущую сессию — можно загрузить пакет заново";
          upload.value = null;
          job.value = null;
          persist();
        }
      }
    } catch (reason) {
      if (currentEpoch === epoch) error.value = errorMessage(reason);
    } finally {
      if (currentEpoch === epoch) initializing.value = false;
    }
  }

  async function selectFile(file: File) {
    if (!capabilities.value || uploading.value || starting.value || isActive(job.value)) return;
    error.value = "";
    if (!/\.siq$/i.test(file.name)) { error.value = "Выберите пакет SIGame с расширением .siq"; return; }
    if (!file.size) { error.value = "Этот файл пустой — выберите другой пакет"; return; }
    if (file.size > capabilities.value.max_upload_bytes) { error.value = "Пакет превышает допустимый размер загрузки"; return; }
    const currentEpoch = ++epoch;
    uploadController?.abort();
    const controller = new AbortController();
    uploadController = controller;
    filename.value = file.name;
    uploadProgress.value = 0;
    uploading.value = true;
    try {
      const pack = await uploadPack(file, controller.signal, value => { if (currentEpoch === epoch) uploadProgress.value = value; });
      if (currentEpoch !== epoch) return;
      upload.value = pack;
      job.value = null;
      defaultsForPack();
      persist();
    } catch (reason) {
      if (currentEpoch === epoch && !controller.signal.aborted) error.value = errorMessage(reason);
    } finally {
      if (currentEpoch === epoch) uploading.value = false;
    }
  }

  function cancelUpload() {
    epoch++;
    uploadController?.abort();
    uploading.value = false;
    filename.value = "";
    uploadProgress.value = 0;
  }

  async function start() {
    if (!upload.value || !options.value || starting.value) return;
    starting.value = true;
    error.value = "";
    const currentEpoch = epoch;
    try {
      const submitted = cloneOptions(options.value);
      // Disabled controls may contain unfinished numeric edits. They are not sent
      // as invalid values because the server validates the entire options object.
      if (capabilities.value) {
        for (const key of ["images", "audio", "video"] as const) {
          if (!submitted[key].enabled) Object.assign(submitted[key], capabilities.value.defaults[key], { enabled: false });
        }
      }
      const next = await startJob(upload.value.id, submitted);
      if (currentEpoch !== epoch) return;
      job.value = next;
      connectionError.value = "";
      failures = 0;
      persist();
      connectUpdates();
    } catch (reason) { if (currentEpoch === epoch) error.value = errorMessage(reason); }
    finally { if (currentEpoch === epoch) starting.value = false; }
  }

  async function cancel() {
    if (!job.value || cancelling.value || !isActive(job.value)) return;
    cancelling.value = true;
    try {
      const next = await cancelJob(job.value.id);
      stopUpdates();
      job.value = next;
      connectionError.value = "";
      persist();
    } catch (reason) { error.value = errorMessage(reason); }
    finally { cancelling.value = false; }
  }

  function editSettings() {
    if (isActive(job.value)) return;
    epoch++;
    stopUpdates();
    job.value = null;
    error.value = "";
    connectionError.value = "";
    persist();
  }

  function reset() {
    if (starting.value || cancelling.value || isActive(job.value)) return;
    cancelUpload();
    stopUpdates();
    upload.value = null;
    job.value = null;
    options.value = null;
    error.value = "";
    connectionError.value = "";
    persist();
  }

  onBeforeUnmount(() => { epoch++; uploadController?.abort(); stopUpdates(); });
  return { capabilities, upload, job, options, stage, initializing, uploading, starting, cancelling,
    uploadProgress, filename, error, connectionError, initialize, selectFile, cancelUpload, start,
    cancel, editSettings, reset, retryStatus, defaultsForPack };
}
