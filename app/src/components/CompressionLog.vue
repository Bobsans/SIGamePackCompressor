<template>
  <section v-if="entries.length" class="compression-log" aria-label="Журнал обработки файлов">
    <div class="compression-log-heading"><h3>Журнал обработки</h3><button v-if="!atTop" type="button" class="log-jump" @click="toTop">К новым записям ↑</button><span v-else>{{ fileCount(log.length) }}</span></div>
    <div class="log-window" :class="{ 'has-more': hasMore }"><div ref="scroller" class="log-scroll" tabindex="0" aria-label="Полный журнал, новые записи сверху" @scroll="measureScroll"><ol class="compression-log-list">
      <li v-for="entry in entries" :key="entry.sequence" class="file-log-row" :class="{ 'is-current': entry.status === 'processing', 'has-warning': !!entry.warning, 'is-interrupted': entry.status === 'interrupted' }" tabindex="0">
        <span class="file-log-marker" aria-hidden="true">{{ entry.status === 'processing' ? '' : entry.status === 'interrupted' ? '–' : entry.warning ? '!' : '✓' }}</span>
        <div class="file-log-description">
          <span class="file-log-name" :title="readableName(entry.name)">{{ readableName(entry.name) }}</span>
          <span class="file-log-status" :title="entry.warning">{{ typeLabel(entry.type) }} <span aria-hidden="true">·</span> {{ statusLabel(entry) }}</span>
        </div>
        <div class="file-log-sizes">
          <span>{{ formatSize(entry.old_size) }}</span>
          <template v-if="entry.new_size !== undefined"><span class="file-log-arrow" aria-hidden="true">→</span><strong>{{ formatSize(entry.new_size) }}</strong></template>
          <span v-else class="file-log-pending">{{ entry.status === 'processing' ? '…' : '—' }}</span>
        </div>
      </li>
    </ol></div></div>
    <p class="visually-hidden" role="status">{{ currentMessage }}</p>
  </section>
</template>

<script setup lang="ts">
  import { computed, nextTick, onMounted, ref, watch } from "vue";
  import type { FileLogEntry } from "@/types";
  import { fileCount, formatSize } from "@/utils";
  const props = defineProps<{ log: FileLogEntry[] }>();
  const entries = computed(() => props.log.slice().reverse());
  const scroller = ref<HTMLDivElement>();
  const atTop = ref(true);
  const hasMore = ref(false);
  function measureScroll() {
    const el = scroller.value;
    if (!el) return;
    atTop.value = el.scrollTop < 12;
    hasMore.value = el.scrollHeight - el.clientHeight - el.scrollTop > 4;
  }
  function toTop() { scroller.value?.scrollTo({ top: 0, behavior: "auto" }); }
  watch(() => props.log.length, async () => {
    const el = scroller.value;
    const top = el?.scrollTop ?? 0;
    const height = el?.scrollHeight ?? 0;
    await nextTick();
    const current = scroller.value;
    if (!current) return;
    // Preserve the reader's position while new rows are prepended.
    current.scrollTop = top < 12 ? 0 : top + (current.scrollHeight - height);
    measureScroll();
  }, { flush: "pre" });
  onMounted(measureScroll);
  function readableName(name: string) { try { return decodeURIComponent(name); } catch { return name; } }
  function typeLabel(type: string) { return ({ image: "Изображение", audio: "Аудио", video: "Видео", html: "HTML" } as Record<string, string>)[type] ?? "Файл"; }
  function statusLabel(entry: FileLogEntry) {
    if (entry.status === "processing") return "Обрабатываем";
    if (entry.status === "interrupted") return "Обработка прервана";
    if (entry.warning) return "Оригинал сохранён · есть замечание";
    return entry.status === "compressed" ? "Сжато" : "Без изменений";
  }
  const currentMessage = computed(() => {
    const entry = entries.value[0];
    return entry ? `${statusLabel(entry)}: ${readableName(entry.name)}` : "";
  });
</script>

<style scoped lang="scss">
  .compression-log { width: min(100%, 700px); margin: 27px auto 24px; padding: 20px 20px 12px; border: 1px solid #e4e5f0; background: #fafaff; border-radius: 9px; text-align: left; }
  .compression-log-heading { display: flex; align-items: center; justify-content: space-between; gap: 14px; margin-bottom: 14px; }
  .compression-log-heading h3 { color: var(--navy); font-size: 12px; font-weight: 600; letter-spacing: 0; }
  .compression-log-heading > span { font-size: 10px; color: var(--muted); }
  .compression-log-list { position: relative; list-style: none; padding: 0; margin: 0; }
  .log-window { position: relative; }
  .log-window::after { content: ""; position: absolute; bottom: 0; left: 0; right: 14px; height: 60px; background: linear-gradient(transparent, #fafaff); pointer-events: none; opacity: 0; transition: opacity .2s; }
  .log-window.has-more::after { opacity: 1; }
  .log-scroll { height: 280px; overflow-y: auto; overflow-x: hidden; overflow-anchor: none; scrollbar-gutter: stable; scrollbar-width: thin; scrollbar-color: #bfc3dd transparent; }
  .log-jump { padding: 0; min-height: 24px; border: 0; background: none; color: var(--navy); font-size: 10px; cursor: pointer; }
  .file-log-row { position: relative; display: flex; align-items: center; gap: 12px; min-height: 50px; padding: 7px 0; border-top: 1px solid #ecedf5; transition: opacity .35s ease, background .35s ease; }
  .file-log-row:first-child { border-top: 0; }
  .file-log-marker { display: grid; place-items: center; width: 19px; height: 19px; flex: none; border-radius: 50%; background: #e9ecea; color: #567360; font-size: 11px; }
  .is-current .file-log-marker { background: transparent; border: 2px solid #d9dcef; border-top-color: var(--navy); animation: log-spin 1s linear infinite; }
  .has-warning .file-log-marker { color: #8a6924; background: #f6ecd3; }
  .is-interrupted .file-log-marker { color: #787d98; background: #e9eaf2; }
  .file-log-description { flex: 1; min-width: 0; }
  .file-log-name { display: block; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; color: #343c68; font-size: 12px; line-height: 1.5; }
  .file-log-row:focus .file-log-name { white-space: normal; overflow-wrap: anywhere; }
  .is-current .file-log-name { color: var(--navy); font-weight: 600; }
  .file-log-status { display: block; color: #797f9a; font-size: 10px; line-height: 1.4; margin-top: 2px; }
  .is-current .file-log-status { color: #656fa4; }
  .file-log-status > span { padding: 0 3px; }
  .file-log-sizes { display: flex; align-items: center; justify-content: flex-end; gap: 9px; flex: none; font-size: 11px; font-variant-numeric: tabular-nums; white-space: nowrap; color: #858ba4; }
  .file-log-sizes strong { font-weight: 600; color: #343c68; }
  .file-log-arrow { color: #afb4ca; }
  .file-log-pending { min-width: 20px; text-align: right; color: var(--navy); }

  @keyframes log-spin { to { transform: rotate(360deg); } }
  @media (max-width: 620px) {
    .compression-log { padding: 16px 12px 8px; }
    .compression-log-heading { align-items: baseline; gap: 8px; }
    .compression-log-heading > span { font-size: 9px; }
    .file-log-row { display: grid; grid-template-columns: 19px minmax(0, 1fr); gap: 4px 9px; }
    .file-log-marker { grid-row: 1 / 3; align-self: start; margin-top: 2px; }
    .file-log-sizes { grid-column: 2; justify-content: flex-start; font-size: 10px; }
    .file-log-name { font-size: 11px; }
  }
  @media (prefers-reduced-motion: reduce) {
    .file-log-row, .log-window::after { transition: none; }
    .is-current .file-log-marker { animation: none; }
  }
</style>
