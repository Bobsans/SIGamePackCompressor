<template>
  <div
    class="progress-bar"
    :class="{ 'is-indeterminate': indeterminate }"
    role="progressbar"
    :aria-label="label"
    :aria-valuemin="0"
    :aria-valuemax="100"
    :aria-valuenow="indeterminate ? undefined : progress"
  >
    <div class="progress-bar-track" :style="{ width: indeterminate ? '35%' : `${progress}%` }" />
  </div>
</template>

<script setup lang="ts">
  import { computed } from "vue";
  const props = withDefaults(defineProps<{ max: number; value: number; label?: string; indeterminate?: boolean }>(), {
    label: "Прогресс обработки", indeterminate: false
  });
  const progress = computed(() => props.max > 0 ? Math.min(100, Math.max(0, Math.round(props.value * 100 / props.max))) : 0);
</script>
