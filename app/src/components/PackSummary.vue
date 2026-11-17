<template>
  <section class="pack-summary" aria-labelledby="pack-title">
    <div class="pack-heading">
      <div class="pack-badge" aria-hidden="true">SIQ<span>{{ pack.analysis.version }}</span></div>
      <div class="pack-name">
        <span class="eyebrow">Ваш пакет</span>
        <h2 id="pack-title">{{ pack.analysis.name || pack.filename }}</h2>
        <p>{{ pack.filename }}</p>
      </div>
      <div class="pack-size"><strong>{{ formatSize(pack.analysis.size) }}</strong><span>исходный размер</span></div>
    </div>
    <dl class="pack-stats">
      <div><dd>{{ pack.analysis.rounds }}</dd><dt>Раунды</dt></div>
      <div><dd>{{ pack.analysis.themes }}</dd><dt>Темы</dt></div>
      <div><dd>{{ pack.analysis.questions }}</dd><dt>Вопросы</dt></div>
      <div><dd>{{ pack.analysis.media.length }}</dd><dt>Медиафайлы</dt></div>
    </dl>
  </section>
  <p v-if="pack.analysis.unused_files" class="field-note cleanup-note">Неиспользуемые вложения в исходном пакете: {{ fileCount(pack.analysis.unused_files) }} · {{ formatSize(pack.analysis.unused_size ?? 0) }}</p>
  <details v-if="pack.analysis.media.length" class="media-details panel">
    <summary><span>Состав пакета <span class="muted">· {{ fileCount(pack.analysis.media.length) }}</span></span><span class="disclosure-icon" aria-hidden="true">+</span></summary>
    <div class="media-breakdown">
      <div v-for="group in groups" :key="group.type"><span class="media-dot" :class="group.type"/><span>{{ group.label }} <b>{{ group.count }}</b></span><strong>{{ formatSize(group.size) }}</strong></div>
    </div>
    <div class="table-scroll">
      <table>
        <caption class="visually-hidden">Файлы пакета и их параметры</caption>
        <thead><tr><th scope="col">Файл</th><th scope="col">Параметры</th><th scope="col">Размер</th></tr></thead>
        <tbody>
          <tr v-for="file in pack.analysis.media" :key="file.name">
            <td><span class="file-extension">{{ file.extension.replace('.', '').toUpperCase() || file.type }}</span><span class="media-name">{{ file.name }}</span></td>
            <td class="media-spec">{{ describe(file.name) }}</td>
            <td>{{ formatSize(file.size) }}</td>
          </tr>
        </tbody>
      </table>
    </div>
    <p class="field-note media-note">Размеры медиа указаны до упаковки в архив</p>
  </details>
  <details v-if="pack.analysis.warnings?.length" class="notice notice-warning">
    <summary>При анализе есть замечания · {{ pack.analysis.warnings.length }}</summary>
    <ul class="warning-list"><li v-for="(warning, i) in pack.analysis.warnings" :key="i">{{ warning }}</li></ul>
  </details>
</template>

<script setup lang="ts">
  import { computed } from "vue";
  import type { UploadedPack } from "@/types";
  import { fileCount, formatSize } from "@/utils";
  const props = defineProps<{ pack: UploadedPack }>();
  const groups = computed(() => [
    { type: "image", label: "Изображения" }, { type: "audio", label: "Аудио" },
    { type: "video", label: "Видео" }, { type: "html", label: "HTML" }
  ].map(group => {
    const files = props.pack.analysis.media.filter(m => m.type === group.type);
    return { ...group, count: files.length, size: files.reduce((sum, m) => sum + m.size, 0) };
  }).filter(group => group.count));
  function describe(name: string) {
    const d = props.pack.media_details?.[name];
    if (!d) return "—";
    const parts: string[] = [];
    if (d.width && d.height) parts.push(`${d.width} × ${d.height}`);
    if (d.frames && d.frames > 1) parts.push(`${d.frames} кадров`);
    if (d.duration_seconds) {
      const seconds = Math.round(d.duration_seconds);
      parts.push(`${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, "0")}`);
    }
    if (d.codec) parts.push(d.codec.toUpperCase());
    return parts.join(" · ") || "—";
  }
</script>
