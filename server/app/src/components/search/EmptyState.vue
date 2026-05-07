<script setup lang="ts">
import icon from "../../assets/icon.svg";
import image from "../../assets/reading.svg";
</script>

<template>
  <div
    class="h-full flex flex-col items-center justify-center gap-5 px-6 text-center">
    <img
      :src="icon"
      alt="openbooks-abs library icon"
      class="h-20 w-20 rounded-full shadow-lg shadow-slate-200/70 ring-1 ring-slate-200 dark:shadow-black/30 dark:ring-slate-700" />
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
