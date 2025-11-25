<template>
  <div class="file-row">
    <div>{{ file.name }}</div>
    <div>
      <progress-bar :max="100" :value="10"/>
    </div>
  </div>
</template>

<script setup lang="ts">
  import ProgressBar from "./ProgressBar.vue";

  const props = defineProps<{
    file: File
  }>();
</script>

<style lang="scss">
  @use "@theme/vars";

  .file-row {
    width: max(30vw, 300px);
    border: 1px solid rgba(vars.$color-glaucous, 0.1);
    border-radius: 5px;
    padding: 5px;
    display: flex;
    flex-direction: column;
  }
</style>
