<template>
  <div
    class="drop-zone"
    :class="{active: state.isDragOver}"
    @click.stop="$input?.click()"
    @drag.prevent=""
    @dragover.prevent=""
    @dragstart="onDragStart"
    @dragenter="onDragStart"
    @dragend="onDragEnd"
    @dragleave="onDragEnd"
    @drop.prevent="onDrop"
  >
    <input ref="$input" type="file" :multiple :accept @input="onSelect">
    <slot/>
  </div>
</template>

<script setup lang="ts">
  import { reactive, ref } from "vue";

  const props = defineProps<{
    multiple?: boolean,
    accept?: string
  }>();

  const emit = defineEmits<{
    (event: "select", data: File | File[]): void
  }>();

  const $input = ref<HTMLInputElement>();

  const model = defineModel<File | File[] | null>();

  const state = reactive({
    isDragOver: false
  });

  const onSelect = (e: Event) => {
    e.preventDefault();
    model.value = props.multiple ? [...$input.value?.files ?? []] : $input.value!.files![0];
    if (model.value) {
      emit("select", model.value);
    }
  };

  const onDrop = (e: DragEvent) => {
    onDragEnd();
    const items = [...e.dataTransfer?.items ?? []].filter((it) => it.kind === "file");

    if (items.length) {
      $input.value!.files = e.dataTransfer?.files ?? null;
      model.value = props.multiple ? [...e.dataTransfer?.files ?? []] : e.dataTransfer?.files[0];
      if (model.value) {
        emit("select", model.value);
      }
    }
  };

  const onDragStart = () => {
    state.isDragOver = true;
  };

  const onDragEnd = () => {
    state.isDragOver = false;
  };
</script>

<style lang="scss">
  @use "@theme/vars";

  .drop-zone {
    width: max(30vw, 300px);
    height: 200px;
    display: flex;
    align-items: center;
    justify-content: center;
    border: 1px dashed gray;
    border-radius: 10px;
    cursor: pointer;
    outline: 0 solid #000050;
    transition: 0.1s ease-in-out;
    will-change: outline-width, background-color, box-shadow;

    &.active {
      background-color: rgba(vars.$color-ink-black, 0.2);
      box-shadow: 0 0 5px 10px rgba(vars.$color-ink-black, 0.1);
    }

    input {
      display: none;
    }
  }
</style>
