<template>
  <div class="progress-bar">
    <div class="progress-bar-track" :style="{width: `${progress * 100}%`}"/>
  </div>
</template>

<script setup lang="ts">
  import { computed } from "vue";

  const props = defineProps<{
    max: number,
    value: number,
  }>();

  const progress = computed(() => props.value / props.max);
</script>

<style lang="scss">
  @use "@theme/vars";

  .progress-bar {
    height: 20px;
    border-radius: 5px;
    background: vars.$color-lavander;
    border: 1px solid rgba(vars.$color-ink-black, 0.1);
    overflow: hidden;

    .progress-bar-track {
      height: 100%;
      background: vars.$color-navy;
    }
  }
</style>
