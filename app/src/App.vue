<template>
  <main>
    <drop-zone v-if="!state.file" v-model="state.file">Drop or choose file</drop-zone>
    <div v-else class="optimize-panel">
      <div>{{ state.file.name }}</div>
      <progress-bar :max="state.total" :value="state.progress"/>
      <div class="log-panel">
        <div v-for="(line, i) in state.log" :key="i">{{ line }}</div>
      </div>
    </div>
  </main>
</template>

<script setup lang="ts">
  import { onMounted, reactive, watch } from "vue";
  import DropZone from "@/components/DropZone.vue";
  import ProgressBar from "@/components/ProgressBar.vue";
  import { compress, wsListen } from "@/api.ts";
  import type { PackInfoSchema } from "@/types.ts";

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
        console.log(data);
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
            window.open(data.url, "_blank");
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
      overflow: auto scroll;
      border: 1px solid rgba(vars.$color-ink-black, 0.1);
      height: 200px;
      border-radius: 5px;
    }
  }
</style>
