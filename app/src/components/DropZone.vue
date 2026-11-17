<template>
  <button
    type="button"
    class="drop-zone"
    :class="{ 'is-over': dragOver }"
    :disabled="disabled"
    @click="input?.click()"
    @dragover.prevent="dragOver = !disabled"
    @dragleave.prevent="dragOver = false"
    @drop.prevent="onDrop"
  >
    <span class="file-mark" aria-hidden="true"><span>SIQ</span><svg viewBox="0 0 24 24" fill="none"><path d="M12 17V5m0 0L7 10m5-5 5 5M5 16v4h14v-4" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/></svg></span>
    <strong>Перетащите пакет сюда</strong>
    <span class="drop-zone-subtitle">или нажмите, чтобы выбрать файл</span>
    <span class="drop-zone-action">Выбрать пакет <span aria-hidden="true">↗</span></span>
    <span class="drop-zone-meta">.siq · версии 3–5<span v-if="maxBytes"> · до {{ formatSize(maxBytes) }}</span></span>
  </button>
  <input ref="input" class="visually-hidden" type="file" accept=".siq" tabindex="-1" aria-label="Файл пакета" :disabled="disabled" @change="onSelect">
</template>

<script setup lang="ts">
  import { ref } from "vue";
  import { formatSize } from "@/utils";
  defineProps<{ disabled: boolean; maxBytes: number }>();
  const emit = defineEmits<{ select: [file: File]; invalid: [message: string] }>();
  const input = ref<HTMLInputElement>();
  const dragOver = ref(false);
  function choose(files: FileList | null) {
    if (!files?.length) return;
    if (files.length !== 1) { emit("invalid", "Загружайте по одному пакету за раз"); return; }
    const file = files.item(0);
    if (file) emit("select", file);
  }
  function onSelect() {
    choose(input.value?.files ?? null);
    if (input.value) input.value.value = "";
  }
  function onDrop(event: DragEvent) {
    dragOver.value = false;
    if (input.value?.disabled) return;
    choose(event.dataTransfer?.files ?? null);
  }
</script>
