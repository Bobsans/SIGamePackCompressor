<template>
  <div class="app-shell">
    <header class="site-header">
      <a class="brand" href="./" aria-label="SIGame Pack Compressor">
        <span class="brand-mark" aria-hidden="true"><i/><i/><i/><i/></span>
        <span>SI<span class="brand-light"> PACK</span><small>Компрессор пакетов</small></span>
      </a>
      <span class="header-caption">Больше игры · меньше мегабайт</span>
    </header>

    <main>
      <div class="page-heading">
        <div><span class="eyebrow">Для SIGame</span><h1 ref="heading" tabindex="-1">{{ titles[stage] }}</h1><p>{{ pageDescription }}</p></div>
        <button v-if="upload && stage === 'settings'" class="button button-secondary button-compact" type="button" :disabled="starting" @click="reset">Другой пакет <span aria-hidden="true">↗</span></button>
      </div>

      <ol class="steps" aria-label="Этапы обработки">
        <li v-for="(label, index) in steps" :key="label" :class="{ current: stepIndex === index, complete: stepIndex > index }" :aria-current="stepIndex === index ? 'step' : undefined">
          <span class="step-number" aria-hidden="true">{{ stepIndex > index ? '✓' : String(index + 1).padStart(2, '0') }}</span><span>{{ label }}</span>
        </li>
      </ol>

      <div v-if="error" class="notice notice-error" role="alert">
        <span>{{ error }}</span>
        <button v-if="!capabilities" class="text-button" type="button" :disabled="initializing" @click="initialize">Попробовать снова</button>
        <button v-else class="dismiss-button" type="button" aria-label="Закрыть сообщение" @click="error = ''">×</button>
      </div>

      <section v-if="initializing" class="loading-panel panel" aria-live="polite">
        <span class="small-spinner" aria-hidden="true"/><h2>Подключаемся к серверу</h2><p>Загружаем настройки и проверяем предыдущую сессию</p>
      </section>

      <template v-else-if="stage === 'upload'">
        <section v-if="uploading" class="uploading-panel panel" aria-live="polite">
          <div class="file-mark small" aria-hidden="true"><span>SIQ</span></div>
          <h2>{{ uploadProgress < 100 ? 'Загружаем пакет' : 'Изучаем содержимое' }}</h2>
          <p class="upload-filename">{{ filename }}</p>
          <div class="upload-progress"><div><span>{{ uploadProgress < 100 ? 'Передача файла' : 'Проверяем вопросы и медиафайлы' }}</span><b v-if="uploadProgress < 100">{{ uploadProgress }}%</b></div><progress-bar :max="100" :value="uploadProgress" :indeterminate="uploadProgress >= 100" label="Загрузка и анализ пакета"/></div>
          <p class="field-note">Сжатие начнётся только после выбора настроек</p>
          <button class="button button-secondary" type="button" @click="cancelUpload">Отменить загрузку</button>
        </section>
        <div v-else class="welcome-grid">
          <section class="upload-panel panel" aria-label="Загрузка пакета">
            <drop-zone :disabled="!capabilities" :max-bytes="capabilities?.max_upload_bytes ?? 0" @select="selectFile" @invalid="error = $event"/>
          </section>
          <aside class="welcome-aside">
            <div class="table-motif" aria-hidden="true"><span>100</span><span>200</span><span>300</span><span>400</span><span>500</span><span class="motif-special">SI</span></div>
            <h2>Тот же пакет<br>Меньше ожидания</h2>
            <p>Уменьшите вес медиа перед игрой — изображения, аудио и видео в одном месте</p>
            <ul class="welcome-points"><li>Настройки для каждого типа медиа</li><li>Обновление формата пакета до версии 5</li><li>Повторная обработка без новой загрузки</li></ul>
          </aside>
        </div>
        <div v-if="!uploading" class="upload-footnote"><span aria-hidden="true">◇</span> Оригинал не изменяется — готовый пакет скачивается отдельным файлом</div>
      </template>

      <template v-else-if="upload">
        <pack-summary :pack="upload"/>
        <compression-settings v-if="stage === 'settings' && options && capabilities" v-model="options" :pack="upload" :capabilities="capabilities" :busy="starting" @start="start" @defaults="defaultsForPack"/>
        <job-panel v-else-if="job" :job="job" :filename="upload.filename" :source-version="upload.analysis.version" :cancelling="cancelling" :connection-error="connectionError" @cancel="cancel" @edit="editSettings" @reset="reset" @retry="retryStatus"/>
      </template>
    </main>
    <footer class="site-footer"><span>SI PACK <span class="footer-separator">/</span> Готовьте вопросы, мы займёмся размером</span><span>JPEG · PNG · GIF · WebP · AVIF</span></footer>
  </div>
</template>

<script setup lang="ts">
  import { computed, nextTick, onMounted, ref, watch } from "vue";
  import DropZone from "@/components/DropZone.vue";
  import ProgressBar from "@/components/ProgressBar.vue";
  import PackSummary from "@/components/PackSummary.vue";
  import CompressionSettings from "@/components/CompressionSettings.vue";
  import JobPanel from "@/components/JobPanel.vue";
  import { useCompression } from "@/composables/useCompression";
  const {
    capabilities, upload, job, options, stage, initializing, uploading, starting, cancelling,
    uploadProgress, filename, error, connectionError, initialize, selectFile, cancelUpload,
    start, cancel, editSettings, reset, retryStatus, defaultsForPack
  } = useCompression();
  const heading = ref<HTMLHeadingElement>();
  const steps = ["Загрузка", "Настройки", "Сжатие", "Результат"];
  const stepIndex = computed(() => ({ upload: 0, settings: 1, processing: 2, result: 3 })[stage.value]);
  const titles = { upload: "Большая игра — компактный пакет", settings: "Всё готово к сжатию", processing: "Пакет в работе", result: "Результат обработки" };
  const descriptions = {
    upload: "Загрузите пакет, выберите настройки и скачайте облегчённую версию",
    settings: "Проверьте состав пакета и выберите подходящий баланс размера и качества",
    processing: "Следите за прогрессом — исходный пакет остаётся без изменений",
    result: "Скачайте результат или попробуйте другие настройки на том же пакете"
  };
  const pageDescription = computed(() => {
    if (stage.value === "result" && job.value?.status === "failed") return "Задание остановилось — проверьте подробности и попробуйте ещё раз";
    if (stage.value === "result" && job.value?.status === "cancelled") return "Можно изменить настройки и запустить обработку заново";
    return descriptions[stage.value];
  });
  watch(stage, async (next, previous) => {
    // Completing a job should not pull a reader away from the file history.
    if (previous === "processing" && next === "result") return;
    await nextTick(); heading.value?.focus({ preventScroll: true }); window.scrollTo({ top: 0, behavior: "instant" });
  });
  onMounted(initialize);
</script>
