<template>
  <form class="settings-form" @submit.prevent="emit('start')">
    <div class="section-heading">
      <div><span class="eyebrow">Подготовка к игре</span><h2>Настройки сжатия</h2></div>
      <button type="button" class="text-button" :disabled="busy" @click="emit('defaults')">Сбросить настройки</button>
    </div>

    <fieldset class="format-panel panel" :disabled="busy">
      <legend class="visually-hidden">Версия выходного пакета</legend>
      <div><h3>Формат пакета</h3><p>Исходная версия <b>{{ pack.analysis.version }}</b></p></div>
      <label class="format-choice" :class="{ selected: model.output_version === '5' }">
        <input v-model="model.output_version" type="radio" value="5" name="output-version">
        <span><strong>Версия 5</strong><small>Актуальный формат</small></span>
      </label>
      <label class="format-choice" :class="{ selected: model.output_version === 'source' }">
        <input v-model="model.output_version" type="radio" value="source" name="output-version">
        <span><strong>Оставить {{ pack.analysis.version }}</strong><small>Без обновления структуры</small></span>
      </label>
    </fieldset>

    <div class="settings-grid">
      <section class="settings-card panel" :class="{ 'is-disabled': !model.images.enabled }" aria-labelledby="images-title">
        <header class="settings-card-header">
          <span class="category-icon" aria-hidden="true"><svg viewBox="0 0 24 24" fill="none"><rect x="3" y="3" width="18" height="18" rx="3"/><circle cx="8" cy="8" r="1.5"/><path d="m3 17 6-6 4 4 3-3 5 5"/></svg></span>
          <div><h3 id="images-title">Изображения</h3><span>{{ fileCount(counts.image) }}</span></div>
          <label class="switch"><input v-model="model.images.enabled" type="checkbox" aria-label="Сжимать изображения" :disabled="busy || !counts.image || !imageAvailable"><span/></label>
        </header>
        <p v-if="counts.image && !imageAvailable" class="unavailable">Обработка изображений сейчас недоступна на сервере</p>
        <fieldset :disabled="busy || !model.images.enabled" aria-labelledby="images-title">
          <label class="field"><span>Формат изображения</span><select v-model="model.images.format"><option value="webp">WebP · компактный</option><option value="source">Оставить исходный</option></select></label>
          <div class="field"><span>Максимальный размер</span><div class="dimension-fields"><label><span class="visually-hidden">Ширина изображения</span><input v-model.number="model.images.max_width" type="number" min="16" max="8192" required aria-label="Ширина изображения"></label><span aria-hidden="true">×</span><label><span class="visually-hidden">Высота изображения</span><input v-model.number="model.images.max_height" type="number" min="16" max="8192" required aria-label="Высота изображения"></label><span class="unit">px</span></div><small>Поле SIGame на Full HD: {{ capabilities.defaults.images.max_width }} × {{ capabilities.defaults.images.max_height }}<br>Уменьшаем пропорционально, без обрезки</small></div>
          <label class="field range-field"><span>Качество <output>{{ model.images.quality }} / 100</output></span><input v-model.number="model.images.quality" aria-label="Качество изображений" type="range" min="1" max="100" :disabled="model.images.format === 'webp' && model.images.lossless"><small>Выше значение — больше деталей и размер файла</small></label>
          <label class="check-field"><input v-model="model.images.lossless" type="checkbox"><span>WebP без потерь качества</span></label>
          <label class="field"><span>Анимация GIF / WebP</span><select v-model="model.images.animation"><option value="preserve">Сохранить исходный формат</option><option value="webp">Сжимать в анимированный WebP</option></select><small>Кадры и время их показа сохраняются</small></label>
        </fieldset>
      </section>

      <section class="settings-card panel" :class="{ 'is-disabled': !model.audio.enabled }" aria-labelledby="audio-title">
        <header class="settings-card-header">
          <span class="category-icon" aria-hidden="true"><svg viewBox="0 0 24 24" fill="none"><path d="M9 18V5l11-2v13M9 9l11-2"/><ellipse cx="6" cy="18" rx="3" ry="3"/><ellipse cx="17" cy="16" rx="3" ry="3"/></svg></span>
          <div><h3 id="audio-title">Аудио</h3><span>{{ fileCount(counts.audio) }}</span></div>
          <label class="switch"><input v-model="model.audio.enabled" type="checkbox" aria-label="Сжимать аудио" :disabled="busy || !counts.audio || !avAvailable"><span/></label>
        </header>
        <p v-if="counts.audio && !avAvailable" class="unavailable">Обработка аудио сейчас недоступна на сервере</p>
        <fieldset :disabled="busy || !model.audio.enabled" aria-labelledby="audio-title">
          <label class="field"><span>Формат аудио</span><select v-model="model.audio.format"><option value="mp3">MP3 · широкая совместимость</option><option value="opus">Opus · компактный</option></select></label>
          <label class="field"><span>Битрейт</span><div class="input-with-unit"><input v-model.number="model.audio.bitrate_kbps" type="number" min="32" max="320" required aria-label="Битрейт аудио"><span>кбит/с</span></div><small>64–96 для речи · 128–192 для музыки</small></label>
          <div class="setting-tip"><span aria-hidden="true">♪</span><p>Для музыкальных вопросов оставьте запас качества — детали могут быть частью ответа</p></div>
        </fieldset>
      </section>

      <section class="settings-card panel" :class="{ 'is-disabled': !model.video.enabled }" aria-labelledby="video-title">
        <header class="settings-card-header">
          <span class="category-icon" aria-hidden="true"><svg viewBox="0 0 24 24" fill="none"><rect x="2" y="5" width="14" height="14" rx="3"/><path d="m16 10 6-4v12l-6-4"/></svg></span>
          <div><h3 id="video-title">Видео</h3><span>{{ fileCount(counts.video) }}</span></div>
          <label class="switch"><input v-model="model.video.enabled" type="checkbox" aria-label="Сжимать видео" :disabled="busy || !counts.video || !avAvailable"><span/></label>
        </header>
        <p v-if="counts.video && !avAvailable" class="unavailable">Обработка видео сейчас недоступна на сервере</p>
        <fieldset :disabled="busy || !model.video.enabled" aria-labelledby="video-title">
          <div class="field"><span>Выходной формат</span><div class="fixed-format">MP4 <span>H.264 + AAC</span></div></div>
          <div class="field"><span>Максимальный размер</span><div class="dimension-fields"><label><span class="visually-hidden">Ширина видео</span><input v-model.number="model.video.max_width" type="number" min="16" max="8192" required aria-label="Ширина видео"></label><span aria-hidden="true">×</span><label><span class="visually-hidden">Высота видео</span><input v-model.number="model.video.max_height" type="number" min="16" max="8192" required aria-label="Высота видео"></label><span class="unit">px</span></div><small>Поле SIGame на Full HD: {{ capabilities.defaults.video.max_width }} × {{ capabilities.defaults.video.max_height }}<br>Уменьшаем пропорционально, без обрезки</small></div>
          <label class="field range-field"><span>Степень сжатия <output>{{ model.video.crf }}</output></span><input v-model.number="model.video.crf" aria-label="Степень сжатия видео" type="range" min="0" max="51"><small>Меньше значение — выше качество и больше файл</small></label>
          <label class="field"><span>Скорость обработки</span><select v-model="model.video.preset"><option v-for="preset in presets" :key="preset.value" :value="preset.value">{{ preset.label }}</option></select><small>Медленнее — эффективнее сжатие</small></label>
          <label class="field"><span>Звук в видео</span><div class="input-with-unit"><input v-model.number="model.video.audio_bitrate_kbps" type="number" min="32" max="320" required aria-label="Битрейт звука в видео"><span>кбит/с</span></div></label>
        </fieldset>
      </section>
    </div>

    <div class="settings-footer">
      <p>Пропорции сохраняются, небольшие изображения и видео не увеличиваются<br>Если перекодированный файл больше исходного, остаётся оригинал</p>
      <div><button class="button button-primary" type="submit" :disabled="busy || !hasChanges">{{ busy ? 'Создаём задание…' : hasCompression ? 'Сжать пакет' : model.output_version === '5' && pack.analysis.version !== '5' ? 'Конвертировать пакет' : 'Очистить пакет' }} <span v-if="!busy" aria-hidden="true">→</span></button><small v-if="!hasChanges" class="field-note">Включите сжатие или измените версию пакета</small></div>
    </div>
  </form>
</template>

<script setup lang="ts">
  import { computed } from "vue";
  import { fileCount } from "@/utils";
  import type { Capabilities, CompressionOptions, UploadedPack } from "@/types";
  const props = defineProps<{ pack: UploadedPack; capabilities: Capabilities; busy: boolean }>();
  const model = defineModel<CompressionOptions>({ required: true });
  const emit = defineEmits<{ start: []; defaults: [] }>();
  const counts = computed(() => ({ image: props.pack.analysis.media.filter(m => m.type === "image").length, audio: props.pack.analysis.media.filter(m => m.type === "audio").length, video: props.pack.analysis.media.filter(m => m.type === "video").length }));
  const imageAvailable = computed(() => props.capabilities.tools.vips && props.capabilities.tools.vipsheader);
  const avAvailable = computed(() => props.capabilities.tools.ffmpeg && props.capabilities.tools.ffprobe);
  const hasCompression = computed(() => (model.value.images.enabled && counts.value.image > 0 && imageAvailable.value) || (model.value.audio.enabled && counts.value.audio > 0 && avAvailable.value) || (model.value.video.enabled && counts.value.video > 0 && avAvailable.value));
  const hasChanges = computed(() => hasCompression.value || (props.pack.analysis.unused_files ?? 0) > 0 || (model.value.output_version === "5" && props.pack.analysis.version !== "5"));
  const presets = [
    { value: "ultrafast", label: "Максимально быстро" }, { value: "superfast", label: "Сверхбыстро" },
    { value: "veryfast", label: "Очень быстро" }, { value: "faster", label: "Быстрее обычного" },
    { value: "fast", label: "Быстро" }, { value: "medium", label: "Баланс скорости и размера" },
    { value: "slow", label: "Медленно" }, { value: "slower", label: "Ещё медленнее" },
    { value: "veryslow", label: "Максимальное сжатие" }
  ];
</script>
