<template>
  <main>
    <drop-zone v-if="!state.file" v-model="state.file">Drop or choose file</drop-zone>
    <div v-else class="optimize-panel">
      <div>{{ state.file.name }}</div>
      <progress-bar :max="state.total" :value="state.progress"/>
      <div class="log-panel">
        <div class="log-panel-content">
          <template v-for="line in state.log" :key="line">
            <transition-zoom-fade direction="bottom">
              <div>{{ line }}</div>
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
  import type { PackInfoSchema } from "@/types";
  import type { AxiosError } from "axios";
  import TransitionZoomFade from "@/components/TransitionZoomFade.vue";
  import { urljoin } from "@/utils";

  const state = reactive<{
    token: string,
    file: File | null,
    log: string[],
    progress: number,
    total: number,
    info: Omit<PackInfoSchema, "type">
  }>({
    token: "",
    file: null,
    log: [],
    progress: 0,
    total: 100,
    info: { items_count: 0, size: 0, version: 0 }
  });

  watch(() => state.file, (nv, ov) => {
    if (nv && nv !== ov) {
      state.log.push("Uploading...");

      wsListen(state.token, (data) => {
        if (data) {
          if (data.type === "log") {
            state.log.unshift(data.content);
            state.progress++;
          } else if (data.type === "error") {
            state.log.unshift(data.error);
          } else if (data.type === "info") {
            state.info = data;
            state.total = data.items_count;
            state.log.unshift(`Package info: ${data.items_count} items, ${data.size} bytes, version ${data.version}`);
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
          state.log[0] = `Uploading: ${percentCompleted}%`;
        }
      }).then((response) => {
        console.log(response);
        state.progress = 0;
      }).catch((error: AxiosError) => {
        console.error(error);
        state.log.unshift(`Error: ${error.message}`);
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
    }

    .log-panel {
      position: relative;
      overflow: hidden;
      height: 400px;

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
