<script setup lang="ts">
import image from "../../assets/reading.svg";
</script>

<template>
  <div
    class="h-full flex flex-col items-center justify-center gap-6 px-6 text-center">
    <h2 class="text-2xl font-light text-slate-500 dark:text-slate-400">
      Search a book to get started.
    </h2>
    <img
      :src="image"
      alt="Person reading"
      class="w-full max-w-md hidden sm:block opacity-80 dark:opacity-60" />
    <img
      :src="image"
      alt="Person reading"
      class="w-48 sm:hidden opacity-80 dark:opacity-60" />
  </div>
</template>
