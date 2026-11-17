<template>
  <section class="job-panel panel" aria-labelledby="job-title">
    <template v-if="active">
      <div class="processing-emblem" aria-hidden="true"><span/><span/><span/></div>
      <span class="eyebrow">{{ job.status === 'queued' ? 'Ожидание' : 'Обработка пакета' }}</span>
      <h2 id="job-title">{{ job.status === 'queued' ? 'Пакет в очереди' : 'Делаем пакет легче' }}</h2>
      <p>{{ job.status === 'queued' ? 'Начнём, как только освободится место' : 'Обрабатываем медиа и собираем готовый архив' }}</p>
      <div class="job-progress"><div><span>{{ job.total ? `Обработано ${job.progress} из ${job.total} файлов` : 'Собираем пакет' }}</span><b>{{ job.total ? `${percent}%` : '' }}</b></div><progress-bar :max="job.total" :value="job.progress" :indeterminate="job.status === 'queued' || job.total === 0 || job.progress >= job.total"/></div>
      <div v-if="connectionError" class="notice notice-warning" role="status"><span>{{ connectionError }}</span><button class="text-button" type="button" @click="emit('retry')">Проверить статус</button></div>
    </template>

    <template v-else-if="job.status === 'completed'">
      <div class="result-emblem" aria-hidden="true">✓</div>
      <span class="eyebrow">Готово к игре</span>
      <h2 id="job-title">{{ savings > 0 ? 'Пакет стал легче' : 'Пакет обработан' }}</h2>
      <p>{{ filename }}</p>
      <div class="result-sizes"><div><span>До обработки</span><strong>{{ formatSize(job.original_size) }}</strong></div><span class="result-arrow" aria-hidden="true">→</span><div><span>После обработки</span><strong>{{ formatSize(job.result_size ?? 0) }}</strong></div><span v-if="savings > 0" class="savings-badge">{{ savings < 1 ? '&lt;1%' : `−${Math.round(savings)}%` }}</span></div>
      <p v-if="savings <= 0" class="result-note">Итоговый архив не стал меньше — можно изменить настройки и попробовать ещё раз</p>
      <p v-else class="result-note">Освободили {{ formatSize(job.original_size - (job.result_size ?? 0)) }} · {{ outputLabel }}</p>
      <p v-if="job.removed_files" class="field-note">Удалено неиспользуемых вложений: {{ fileCount(job.removed_files) }} · {{ formatSize(job.removed_bytes ?? 0) }} до упаковки</p>
      <div class="result-actions"><a v-if="job.download_url" class="button button-primary" :href="makeUrl(job.download_url)" download>Скачать пакет <span aria-hidden="true">↓</span></a><button class="button button-secondary" type="button" @click="emit('edit')">Изменить настройки</button></div>
      <button class="text-button new-pack-button" type="button" @click="emit('reset')">Загрузить другой пакет <span aria-hidden="true">↗</span></button>
    </template>

    <template v-else>
      <div class="result-emblem" :class="{ 'is-error': job.status === 'failed' }" aria-hidden="true">{{ job.status === 'failed' ? '!' : 'Ⅱ' }}</div>
      <h2 id="job-title">{{ job.status === 'failed' ? 'Не удалось обработать пакет' : 'Обработка отменена' }}</h2>
      <p>Исходный пакет сохранён — его не нужно загружать повторно</p>
      <details v-if="job.error" class="notice notice-warning"><summary>Подробности ошибки</summary><p class="error-detail">{{ job.error }}</p></details>
      <div class="result-actions"><button class="button button-primary" type="button" @click="emit('edit')">Вернуться к настройкам</button><button class="button button-secondary" type="button" @click="emit('reset')">Другой пакет</button></div>
    </template>

    <compression-log :log="job.log ?? []"/>
    <template v-if="active">
      <p class="field-note">Перезагрузка вкладки не прервёт обработку</p>
      <button class="button button-secondary" type="button" :disabled="cancelling" @click="emit('cancel')">{{ cancelling ? 'Отменяем…' : 'Отменить обработку' }}</button>
    </template>
    <details v-if="job.warnings?.length" class="notice notice-warning job-warnings"><summary>Замечания при обработке · {{ job.warnings.length }}</summary><p>Часть файлов могла остаться без изменений</p><ul class="warning-list"><li v-for="(warning, i) in job.warnings" :key="i">{{ warning }}</li></ul></details>
  </section>
</template>

<script setup lang="ts">
  import { computed } from "vue";
  import { makeUrl } from "@/api";
  import type { CompressionJob } from "@/types";
  import { fileCount, formatSize } from "@/utils";
  import ProgressBar from "./ProgressBar.vue";
  import CompressionLog from "./CompressionLog.vue";
  const props = defineProps<{ job: CompressionJob; filename: string; sourceVersion: string; cancelling: boolean; connectionError: string }>();
  const emit = defineEmits<{ cancel: []; edit: []; reset: []; retry: [] }>();
  const active = computed(() => props.job.status === "queued" || props.job.status === "running");
  const percent = computed(() => props.job.total ? Math.min(100, Math.round(props.job.progress * 100 / props.job.total)) : 0);
  const savings = computed(() => props.job.original_size ? (1 - (props.job.result_size ?? props.job.original_size) / props.job.original_size) * 100 : 0);
  const outputLabel = computed(() => `Версия пакета ${props.job.options.output_version === "source" ? props.sourceVersion : props.job.options.output_version}`);
</script>
