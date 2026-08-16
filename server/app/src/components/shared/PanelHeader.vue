<template>
  <div
    class="flex-shrink-0 flex items-center justify-between px-3 py-2
           border-b border-slate-100 dark:border-slate-800">
    <span class="text-xs font-medium text-slate-500 dark:text-slate-400">
      <slot />
    </span>
    <slot name="actions" />
  </div>
</template>
