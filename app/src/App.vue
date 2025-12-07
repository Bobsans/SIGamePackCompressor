<template>
  <main>
    <drop-zone v-if="!state.file" v-model="state.file">Drop or choose file</drop-zone>
    <div v-else class="optimize-panel">
      <div class="pack-info">
        <div>{{ state.file.name }}</div>
        <div v-if="state.info" class="pack-info-data">
          <span>Version: <b>{{ state.info.version }}</b></span>
          <span>Size: <b>{{ formatSize(state.info.size) }}</b></span>
          <span>Items: <b>{{ state.info.items_count }}</b></span>
        </div>
      </div>
      <progress-bar :max="state.total" :value="state.progress"/>
      <div class="log-panel">
        <div class="log-panel-content">
          <template v-for="entry in state.log" :key="entry.id">
            <transition-zoom-fade direction="bottom">
              <div v-if="entry.event === 'compressed'" class="log-entry-file">
                <span class="info">Processed {{ entry.type.padEnd(5, "&nbsp") }} : <b>{{ entry.old_name }}</b> -> <b>{{ entry.new_name }}</b></span>
                <span class="sizes">&nbsp;[<b>{{ formatSize(entry.old_size) }}</b> -> <b>{{ formatSize(entry.new_size) }}</b>]</span>
              </div>
              <div v-else-if="entry.event === 'uploading'">Uploading: {{ entry.percent }}%...</div>
              <div v-else-if="entry.event === 'error'" class="log-entry-error">
                <span>Error: {{ entry.error }}</span>
                <span v-if="entry.type || entry.name || entry.size">[type: {{ entry.type }}, name: {{ entry.name }}, size: {{ formatSize(entry.size ?? 0) }}]</span>
              </div>
            </transition-zoom-fade>
          </template>
        </div>
      </div>
    </div>
  </main>
</template>

<script setup lang="ts">
  import { onMounted, reactive, watch } from "vue";
  import DropZone from "@/components/DropZone.vue";
  import ProgressBar from "@/components/ProgressBar.vue";
  import { API_URL, compress, wsListen } from "@/api";
  import type { LogEntryDataType, LogEntryUploading, PackInfoSchema } from "@/types";
  import type { AxiosError } from "axios";
  import TransitionZoomFade from "@/components/TransitionZoomFade.vue";
  import { formatSize, urljoin } from "@/utils";

  const state = reactive({
    token: "",
    file: null as File | null,
    log: [] as LogEntryDataType[],
    progress: 0,
    total: 100,
    info: null as PackInfoSchema | null
  });

  watch(() => state.file, (nv, ov) => {
    if (nv && nv !== ov) {
      state.log.push({ event: "uploading", percent: 0 });

      wsListen(state.token, (data) => {
        if (data) {
          if (data.type === "log") {
            state.log.unshift({ ...data.data, id: crypto.randomUUID() });
            state.progress++;
          } else if (data.type === "info") {
            state.info = data;
            state.total = data.items_count;
          } else if (data.type === "result") {
            window.open(urljoin(API_URL, data.url), "_blank");
          }
        }
      });

      compress(state.file!, {
        params: { token: state.token },
        onUploadProgress: (e) => {
          const percentCompleted = Math.round((e.loaded * 100) / e.total!);
          state.progress = percentCompleted;
          (state.log[0] as LogEntryUploading).percent = percentCompleted;
        }
      }).then((response) => {
        console.log(response);
        state.progress = 0;
      }).catch((error: AxiosError) => {
        console.error(error);
        state.log.unshift({ event: "error", error: error.message });
      });
    }
  });

  onMounted(() => {
    state.token = localStorage.getItem("token") ?? "";
    if (!state.token) {
      state.token = crypto.randomUUID().replace(/-/g, "");
      localStorage.setItem("token", state.token);
    }
  });
</script>

<style lang="scss">
  @use "@theme/vars";

  main {
    padding: 1.5em;
    display: flex;
    flex-direction: column;
    gap: 20px;

    .optimize-panel {
      width: max(80vw, 300px);
      display: flex;
      flex-direction: column;
      gap: 10px;

      .pack-info {
        display: flex;
        justify-content: space-between;
        font-size: 1.2rem;

        .pack-info-data {
          display: flex;
          gap: 10px;
        }
      }
    }

    .log-panel {
      position: relative;
      overflow: hidden;
      height: 400px;
      font-family: "Consolas", "Menlo", "DejaVu Sans Mono", "Bitstream Vera Sans Mono", monospace;

      &-content {
        position: relative;
        overflow: auto scroll;
        height: 100%;

        &::-webkit-scrollbar-corner {
          background: vars.$color-navy;
          border-radius: 3px;
        }

        &::-webkit-scrollbar {
          background: transparent;
          width: 5px;
          height: 5px;

          &-thumb {
            background: vars.$color-navy;
            border-radius: 3px;
          }
        }

        &::after {
          content: "";
          display: block;
          height: 150px;
        }

        .log-entry-file {
          .info {
            b {
              color: vars.$color-glaucous;
            }
          }

          .sizes {
            color: vars.$color-ink-black;
          }
        }

        .log-entry-error {
          color: red;
        }
      }

      &::after {
        content: "";
        display: block;
        position: absolute;
        bottom: 0;
        left: 0;
        right: 5px;
        height: 150px;
        background: linear-gradient(180deg, transparent, vars.$color-lavander 90%);
        pointer-events: none;
      }
    }
  }
</style>
