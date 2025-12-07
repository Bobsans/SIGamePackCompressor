<template>
  <transition :name="`zoom-fade-${direction}`" :mode="mode" :appear="appear">
    <slot/>
  </transition>
</template>

<script setup lang="ts">
  withDefaults(defineProps<{
    direction?: "top" | "bottom" | "left" | "right",
    mode?: "default" | "out-in" | "in-out",
    appear?: boolean
  }>(), {
    direction: "top",
    mode: "out-in",
    appear: true
  });
</script>

<style lang="scss">
  $timing: 300ms linear 100ms;

  .zoom-fade {
    &-top, &-bottom, &-left, &-right {
      &-enter-active, &-leave-active {
        opacity: 1;
        transition: max-height $timing, transform $timing, opacity $timing;
      }

      &-enter-from, &-leave-active {
        opacity: 0;
      }
    }

    &-top, &-bottom {
      &-enter-active, &-leave-active {
        max-height: 50px;
        transform: scaleY(1);
      }

      &-enter-from, &-leave-active {
        max-height: 0;
        transform: scaleY(0);
      }
    }

    &-left, &-right {
      &-enter-active, &-leave-active {
        max-height: 50px;
        transform: scaleX(1);
      }

      &-enter-from, &-leave-active {
        max-height: 0;
        transform: scaleX(0);
      }
    }

    &-top {
      &-enter-active, &-leave-active {
        transform-origin: center top;
      }
    }

    &-bottom {
      &-enter-active, &-leave-active {
        transform-origin: center bottom;
      }
    }

    &-left {
      &-enter-active, &-leave-active {
        transform-origin: left center;
      }
    }

    &-right {
      &-enter-active, &-leave-active {
        transform-origin: right center;
      }
    }
  }
</style>
