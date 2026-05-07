<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { useAppStore } from "../stores/app";
import { MessageType, type RenameOption } from "../types/messages";
import { sendMessage } from "../composables/useWebSocket";

const appStore = useAppStore();

// Editable metadata fields — initialised from server payload when modal opens.
const editAuthor = ref("");
const editTitle = ref("");
const editSeries = ref("");
const editSeriesIndex = ref("");
const selectedId = ref("keep");
const customName = ref("");
const rewriteMetadata = ref(false);

// Reset state whenever a new rename prompt arrives.
watch(
  () => appStore.pendingRename,
  (prompt) => {
    if (!prompt) return;
    editAuthor.value = prompt.metadata?.Author ?? "";
    editTitle.value = prompt.metadata?.Title ?? "";
    editSeries.value = prompt.metadata?.Series ?? "";
    editSeriesIndex.value = prompt.metadata?.SeriesIndex ?? "";
    // Default to the best organized option available.
    const ids = prompt.options.map((o) => o.id);
    if (ids.includes("series")) selectedId.value = "series";
    else if (ids.includes("organized")) selectedId.value = "organized";
    else selectedId.value = "keep";
    customName.value = "";
    rewriteMetadata.value = false;
  }
);

const prompt = computed(() => appStore.pendingRename);

// Sanitize a path component the same way the Go backend does.
function sanitize(s: string, replaceSpace: string): string {
  s = s.trim().replace(/[/\\]/g, "-");
  if (replaceSpace) s = s.replace(/ /g, replaceSpace);
  return s;
}

// Recompute option previews from the (possibly edited) metadata fields.
// Mirrors buildRenameOptions() in server/staging.go.
const liveOptions = computed((): RenameOption[] => {
  const p = prompt.value;
  if (!p) return [];

  const rs = p.replaceSpace;
  const irc = p.ircFilename;
  const isEPUB = irc.toLowerCase().endsWith(".epub");
  const ext = irc.slice(irc.lastIndexOf(".")).toLowerCase();

  const author = sanitize(editAuthor.value, rs);
  const title = sanitize(editTitle.value, rs);
  const series = sanitize(editSeries.value, rs);

  const opts: RenameOption[] = [
    { id: "keep", label: "Keep IRC filename", preview: irc, isOrganized: false },
  ];

  if (!isEPUB || !title) return opts;

  opts.push({ id: "title", label: "Title only", preview: `${title}${ext}`, isOrganized: false });

  if (author) {
    opts.push({
      id: "author-title-flat",
      label: "Author — Title (flat)",
      preview: `${author} - ${title}${ext}`,
      isOrganized: false,
    });
    opts.push({
      id: "organized",
      label: "Author / Title /",
      preview: `${author}/${title}/${title}${ext}`,
      isOrganized: true,
    });
    if (series) {
      opts.push({
        id: "series",
        label: "Author / Series / Title /",
        preview: `${author}/${series}/${title}/${title}${ext}`,
        isOrganized: true,
      });
    }
  }

  return opts;
});

// Ensure selectedId stays valid when options change (e.g. user clears author).
watch(liveOptions, (opts) => {
  if (!opts.find((o) => o.id === selectedId.value)) {
    selectedId.value = opts[opts.length - 1]?.id ?? "keep";
  }
});

const hasMetadata = computed(
  () => !!(prompt.value?.metadata?.Title || prompt.value?.metadata?.Author)
);

// True if the metadata fields were populated from the EPUB (not just empty defaults).
const hasEmbeddedMetadata = computed(() => !!prompt.value?.metadata);

const metadataEdited = computed(
  () =>
    editAuthor.value !== (prompt.value?.metadata?.Author ?? "") ||
    editTitle.value !== (prompt.value?.metadata?.Title ?? "") ||
    editSeries.value !== (prompt.value?.metadata?.Series ?? "") ||
    editSeriesIndex.value !== (prompt.value?.metadata?.SeriesIndex ?? "")
);

// Auto-enable rewrite when the user edits any metadata field.
watch(metadataEdited, (edited) => {
  if (edited) rewriteMetadata.value = true;
});

function confirm() {
  const p = prompt.value;
  if (!p) return;
  sendMessage({
    type: MessageType.RENAME_CONFIRM,
    payload: {
      optionId: selectedId.value,
      customName: customName.value,
      rewriteMetadata: rewriteMetadata.value,
      author: editAuthor.value,
      title: editTitle.value,
      series: editSeries.value,
      seriesIndex: editSeriesIndex.value,
    },
  });
  appStore.pendingRename = null;
}

function cancel() {
  sendMessage({
    type: MessageType.RENAME_CONFIRM,
    payload: {
      optionId: "keep",
      customName: "",
      rewriteMetadata: false,
      author: "",
      title: "",
      series: "",
      seriesIndex: "",
    },
  });
  appStore.pendingRename = null;
}
</script>

<template>
  <Transition name="modal">
    <div
      v-if="prompt"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm"
      @click.self="cancel"
    >
      <div
        class="relative w-full max-w-2xl max-h-[90vh] flex flex-col bg-white dark:bg-slate-900 rounded-2xl shadow-2xl overflow-hidden"
      >
        <!-- Header -->
        <div class="px-6 pt-6 pb-4 border-b border-slate-200 dark:border-slate-700 flex gap-4 items-start">
          <!-- Cover thumbnail -->
          <img
            v-if="prompt.coverBase64"
            :src="`data:${prompt.coverMime};base64,${prompt.coverBase64}`"
            alt="Book cover"
            class="flex-shrink-0 w-16 h-24 object-cover rounded shadow-md ring-1 ring-slate-200 dark:ring-slate-700"
          />
          <div class="min-w-0">
            <h2 class="text-lg font-semibold text-slate-900 dark:text-slate-50">
              Book Ready to Save
            </h2>
            <p class="mt-1 text-sm text-slate-500 dark:text-slate-400 font-mono break-all">
              {{ prompt.ircFilename }}
            </p>
          </div>
        </div>

        <!-- Scrollable body -->
        <div class="flex-1 overflow-y-auto px-6 py-4 space-y-5">

          <!-- Metadata editor -->
          <div>
            <div class="flex items-center justify-between mb-2 gap-2 flex-wrap">
              <h3 class="text-sm font-medium text-slate-700 dark:text-slate-300">Metadata</h3>
              <div class="flex items-center gap-2">
                <span
                  v-if="hasEmbeddedMetadata"
                  class="text-[10px] px-1.5 py-0.5 rounded-full bg-emerald-100 dark:bg-emerald-900/40 text-emerald-700 dark:text-emerald-400 font-medium"
                  title="Values were read from the EPUB file itself"
                >📖 embedded</span>
                <span
                  v-else
                  class="text-[10px] px-1.5 py-0.5 rounded-full bg-slate-100 dark:bg-slate-800 text-slate-500 dark:text-slate-400 font-medium"
                  title="No embedded metadata — values parsed from the IRC filename"
                >📄 IRC filename</span>
                <span
                  v-if="metadataEdited"
                  class="text-[10px] px-1.5 py-0.5 rounded-full bg-blue-100 dark:bg-blue-900/40 text-blue-700 dark:text-blue-400 font-medium"
                >edited</span>
                <span
                  v-else-if="!hasMetadata"
                  class="text-xs text-amber-600 dark:text-amber-400"
                >No metadata found — options limited</span>
              </div>
            </div>
            <div class="grid grid-cols-[5rem_1fr] gap-x-3 gap-y-2 items-center">
              <label class="text-xs text-slate-500 dark:text-slate-400 text-right">Author</label>
              <input
                v-model="editAuthor"
                type="text"
                placeholder="Unknown author"
                class="w-full px-3 py-1.5 text-sm rounded-lg border border-slate-300 dark:border-slate-600 bg-white dark:bg-slate-800 text-slate-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
              <label class="text-xs text-slate-500 dark:text-slate-400 text-right">Title</label>
              <input
                v-model="editTitle"
                type="text"
                placeholder="Unknown title"
                class="w-full px-3 py-1.5 text-sm rounded-lg border border-slate-300 dark:border-slate-600 bg-white dark:bg-slate-800 text-slate-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
              <label class="text-xs text-slate-500 dark:text-slate-400 text-right self-start pt-2">Series</label>
              <div class="space-y-2">
                <input
                  v-model="editSeries"
                  type="text"
                  placeholder="(none)"
                  class="w-full px-3 py-1.5 text-sm rounded-lg border border-slate-300 dark:border-slate-600 bg-white dark:bg-slate-800 text-slate-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
                <div class="flex items-center gap-2">
                  <label class="text-xs text-slate-400 dark:text-slate-500 whitespace-nowrap"># in series</label>
                  <input
                    v-model="editSeriesIndex"
                    type="text"
                    inputmode="decimal"
                    placeholder="1"
                    class="w-16 px-2 py-1.5 text-sm rounded-lg border border-slate-300 dark:border-slate-600 bg-white dark:bg-slate-800 text-slate-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-blue-500 text-center"
                  />
                </div>
              </div>
            </div>
          </div>

          <!-- Save-as options -->
          <div>
            <h3 class="text-sm font-medium text-slate-700 dark:text-slate-300 mb-2">Save as</h3>
            <div class="space-y-2">
              <label
                v-for="opt in liveOptions"
                :key="opt.id"
                class="flex items-start gap-3 p-3 rounded-xl border cursor-pointer transition-colors"
                :class="
                  selectedId === opt.id
                    ? opt.id === 'series'
                      ? 'border-violet-500 bg-violet-50 dark:bg-violet-950/40'
                      : 'border-blue-500 bg-blue-50 dark:bg-blue-950/40'
                    : 'border-slate-200 dark:border-slate-700 hover:border-slate-300 dark:hover:border-slate-600'
                "
              >
                <input
                  type="radio"
                  :value="opt.id"
                  v-model="selectedId"
                  class="mt-0.5 shrink-0"
                  :class="opt.id === 'series' ? 'accent-violet-500' : 'accent-blue-500'"
                />
                <div class="min-w-0 flex-1">
                  <div class="flex items-center gap-2 flex-wrap">
                    <span class="text-sm font-medium text-slate-800 dark:text-slate-200">
                      {{ opt.label }}
                    </span>
                    <span
                      v-if="opt.id === 'series'"
                      class="text-[10px] px-1.5 py-0.5 rounded-full bg-violet-100 dark:bg-violet-900/50 text-violet-700 dark:text-violet-400 font-medium"
                    >with series</span>
                    <span
                      v-else-if="opt.isOrganized"
                      class="text-[10px] px-1.5 py-0.5 rounded-full bg-emerald-100 dark:bg-emerald-900/50 text-emerald-700 dark:text-emerald-400 font-medium"
                    >organized</span>
                  </div>
                  <!-- Folder path visualization for organized options -->
                  <div v-if="opt.isOrganized" class="mt-1.5 flex items-center gap-1 flex-wrap text-[11px] font-mono">
                    <template v-for="(seg, si) in opt.preview.split('/')" :key="si">
                      <span
                        :class="si === opt.preview.split('/').length - 1
                          ? 'text-slate-600 dark:text-slate-300'
                          : opt.id === 'series' ? 'text-violet-600 dark:text-violet-400 font-semibold' : 'text-emerald-600 dark:text-emerald-400 font-semibold'"
                      >{{ seg }}</span>
                      <span v-if="si < opt.preview.split('/').length - 1" class="text-slate-300 dark:text-slate-600">/</span>
                    </template>
                  </div>
                  <p v-else class="mt-0.5 text-xs font-mono text-slate-500 dark:text-slate-400 break-all">
                    {{ opt.preview }}
                  </p>
                </div>
              </label>

              <!-- Custom option -->
              <label
                class="flex items-start gap-3 p-3 rounded-xl border cursor-pointer transition-colors"
                :class="
                  selectedId === 'custom'
                    ? 'border-blue-500 bg-blue-50 dark:bg-blue-950/40'
                    : 'border-slate-200 dark:border-slate-700 hover:border-slate-300 dark:hover:border-slate-600'
                "
              >
                <input
                  type="radio"
                  value="custom"
                  v-model="selectedId"
                  class="mt-0.5 accent-blue-500 shrink-0"
                />
                <div class="flex-1 min-w-0">
                  <span class="text-sm font-medium text-slate-800 dark:text-slate-200">Custom filename</span>
                  <input
                    v-model="customName"
                    type="text"
                    placeholder="my-book.epub"
                    class="mt-1.5 w-full px-3 py-1.5 text-sm font-mono rounded-lg border border-slate-300 dark:border-slate-600 bg-white dark:bg-slate-800 text-slate-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
                    @focus="selectedId = 'custom'"
                  />
                </div>
              </label>
            </div>
          </div>

          <!-- Rewrite metadata toggle — only meaningful for EPUBs -->
          <div
            v-if="prompt.ircFilename.toLowerCase().endsWith('.epub')"
            class="flex items-start gap-3 p-3 rounded-xl border transition-colors"
            :class="rewriteMetadata
              ? 'border-blue-400 dark:border-blue-600 bg-blue-50/50 dark:bg-blue-950/20'
              : 'border-slate-200 dark:border-slate-700'"
          >
            <input
              id="rewrite-toggle"
              type="checkbox"
              v-model="rewriteMetadata"
              class="mt-0.5 accent-blue-500 shrink-0"
            />
            <label for="rewrite-toggle" class="cursor-pointer">
              <div class="text-sm font-medium text-slate-800 dark:text-slate-200">
                Rewrite EPUB internal metadata
              </div>
              <div class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                <template v-if="metadataEdited">
                  ✏️ You edited the fields above — this will write your changes into the EPUB.
                </template>
                <template v-else-if="hasEmbeddedMetadata">
                  Metadata already matches the embedded values. Only enable if you want to force a rewrite.
                </template>
                <template v-else>
                  No embedded metadata found. Enable to write Author, Title, and Series into the EPUB.
                </template>
              </div>
            </label>
          </div>
        </div>

        <!-- Footer -->
        <div class="px-6 py-4 border-t border-slate-200 dark:border-slate-700 flex items-center justify-between gap-3">
          <button
            @click="cancel"
            class="px-4 py-2 text-sm rounded-lg border border-slate-300 dark:border-slate-600 text-slate-700 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-800 transition-colors"
          >
            Keep IRC filename
          </button>
          <button
            @click="confirm"
            class="px-5 py-2 text-sm font-medium rounded-lg bg-blue-600 hover:bg-blue-700 text-white transition-colors"
          >
            Save Book
          </button>
        </div>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.modal-enter-active,
.modal-leave-active {
  transition: opacity 0.2s ease;
}
.modal-enter-active .relative,
.modal-leave-active .relative {
  transition: transform 0.2s ease, opacity 0.2s ease;
}
.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}
.modal-enter-from .relative,
.modal-leave-to .relative {
  transform: scale(0.96);
  opacity: 0;
}
</style>
