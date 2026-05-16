<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { Trash2 } from "lucide-vue-next";
import { useAppStore } from "../stores/app";
import { MessageType, type RenameOption } from "../types/messages";
import { sendMessage } from "../composables/useWebSocket";

const appStore = useAppStore();

const editAuthor = ref("");
const editTitle = ref("");
const editSeries = ref("");
const editSeriesIndex = ref("");
const editFileName = ref("");
const fileNameEdited = ref(false);
const selectedId = ref("keep");
const customName = ref("");
const rewriteMetadata = ref(false);

watch(
  () => appStore.pendingStagedBook,
  (book) => {
    if (!book) return;
    editAuthor.value = book.metadata?.Author ?? "";
    editTitle.value = book.metadata?.Title ?? "";
    editSeries.value = book.metadata?.Series ?? "";
    editSeriesIndex.value = book.metadata?.SeriesIndex ?? "";
    fileNameEdited.value = false;
    editFileName.value = defaultFileName(book.ircFilename, editTitle.value, book.replaceSpace);
    const ids = book.options.map((o) => o.id);
    if (ids.includes("series")) selectedId.value = "series";
    else if (ids.includes("organized")) selectedId.value = "organized";
    else selectedId.value = "keep";
    customName.value = "";
    rewriteMetadata.value = false;
  }
);

const book = computed(() => appStore.pendingStagedBook);

function sanitize(s: string, replaceSpace: string): string {
  s = s.trim().replace(/[/\\]/g, "-");
  if (replaceSpace) s = s.replace(/ /g, replaceSpace);
  return s;
}

function extension(filename: string): string {
  const idx = filename.lastIndexOf(".");
  return idx === -1 ? "" : filename.slice(idx).toLowerCase();
}

function defaultFileName(ircFilename: string, title: string, replaceSpace: string): string {
  const ext = extension(ircFilename);
  const safeTitle = sanitize(title, replaceSpace);
  return safeTitle ? `${safeTitle}${ext}` : ircFilename;
}

function fileNameWithExtension(filename: string, ext: string): string {
  const trimmed = filename.trim();
  if (!trimmed) return "";
  return extension(trimmed) ? trimmed : `${trimmed}${ext}`;
}

const liveOptions = computed((): RenameOption[] => {
  const b = book.value;
  if (!b) return [];

  const rs = b.replaceSpace;
  const irc = b.ircFilename;
  const isEPUB = irc.toLowerCase().endsWith(".epub");
  const ext = extension(irc);

  const author = sanitize(editAuthor.value, rs);
  const title = sanitize(editTitle.value, rs);
  const series = sanitize(editSeries.value, rs);
  const fileName = sanitize(fileNameWithExtension(editFileName.value, ext), rs);

  const opts: RenameOption[] = [
    { id: "keep", label: "Keep IRC filename", preview: irc, isOrganized: false },
  ];

  if (!isEPUB || !title) return opts;

  opts.push({ id: "title", label: "Title only", preview: fileName || `${title}${ext}`, isOrganized: false });

  if (author) {
    opts.push({
      id: "author-title-flat",
      label: "Author — Title (flat)",
      preview: `${author} - ${fileName || `${title}${ext}`}`,
      isOrganized: false,
    });
    opts.push({
      id: "organized",
      label: "Author / Title /",
      preview: `${author}/${title}/${fileName || `${title}${ext}`}`,
      isOrganized: true,
    });
    if (series) {
      opts.push({
        id: "series",
        label: "Author / Series / Title /",
        preview: `${author}/${series}/${title}/${fileName || `${title}${ext}`}`,
        isOrganized: true,
      });
    }
  }

  return opts;
});

watch(liveOptions, (opts) => {
  if (!opts.find((o) => o.id === selectedId.value)) {
    selectedId.value = opts[opts.length - 1]?.id ?? "keep";
  }
});

watch(editSeries, () => {
  if (liveOptions.value.find((o) => o.id === "series")) {
    selectedId.value = "series";
  }
  if (editSeries.value.trim() && !editSeriesIndex.value.trim()) {
    editSeriesIndex.value = "0";
  }
});

watch(editTitle, () => {
  const b = book.value;
  if (b && !fileNameEdited.value) {
    editFileName.value = defaultFileName(b.ircFilename, editTitle.value, b.replaceSpace);
  }
});

const hasMetadata = computed(
  () => !!(book.value?.metadata?.Title || book.value?.metadata?.Author)
);

const hasEmbeddedMetadata = computed(() => !!book.value?.metadata);

const metadataEdited = computed(
  () =>
    editAuthor.value !== (book.value?.metadata?.Author ?? "") ||
    editTitle.value !== (book.value?.metadata?.Title ?? "") ||
    editSeries.value !== (book.value?.metadata?.Series ?? "") ||
    editSeriesIndex.value !== (book.value?.metadata?.SeriesIndex ?? "")
);

watch(metadataEdited, (edited) => {
  if (edited) rewriteMetadata.value = true;
});

const stagedAt = computed(() => {
  const raw = book.value?.stagedAt;
  if (!raw) return null;
  const d = new Date(raw);
  const diffMs = Date.now() - d.getTime();
  const diffDays = Math.floor(diffMs / 86400000);
  if (diffDays === 0) return "staged today";
  if (diffDays === 1) return "staged yesterday";
  return `staged ${diffDays} days ago`;
});

function confirm() {
  const b = book.value;
  if (!b) return;
  sendMessage({
    type: MessageType.RENAME_CONFIRM,
    payload: {
      optionId: selectedId.value,
      customName: customName.value,
      fileName: editFileName.value,
      rewriteMetadata: rewriteMetadata.value,
      author: editAuthor.value,
      title: editTitle.value,
      series: editSeries.value,
      seriesIndex: editSeriesIndex.value,
      stagedId: b.stagedId,
    },
  });
  appStore.setPendingStagedBook(null);
}

function saveLater() {
  const b = book.value;
  if (!b) return;
  sendMessage({
    type: MessageType.STAGED_QUEUE_LATER,
    payload: { stagedId: b.stagedId },
  });
  appStore.setPendingStagedBook(null);
}

const confirmingDelete = ref(false);

function deleteStaged() {
  const b = book.value;
  if (!b) return;
  sendMessage({
    type: MessageType.DELETE_STAGED,
    payload: { stagedId: b.stagedId },
  });
  appStore.setPendingStagedBook(null);
  confirmingDelete.value = false;
}
</script>

<template>
  <Transition name="modal">
    <div
      v-if="book"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm"
      @click.self="saveLater"
    >
      <div
        class="relative w-full max-w-2xl max-h-[90vh] flex flex-col bg-white dark:bg-slate-900 rounded-2xl shadow-2xl overflow-hidden"
      >
        <!-- Header -->
        <div class="px-6 pt-6 pb-4 border-b border-slate-200 dark:border-slate-700 flex gap-4 items-start">
          <img
            v-if="book.coverBase64"
            :src="`data:${book.coverMime};base64,${book.coverBase64}`"
            alt="Book cover"
            class="flex-shrink-0 w-16 h-24 object-cover rounded shadow-md ring-1 ring-slate-200 dark:ring-slate-700"
          />
          <div class="min-w-0 flex-1">
            <div class="flex items-center justify-between gap-2 flex-wrap">
              <h2 class="text-lg font-semibold text-slate-900 dark:text-slate-50">
                Staged Book
              </h2>
              <div class="flex items-center gap-2">
                <span
                  v-if="stagedAt"
                  class="text-xs text-slate-400 dark:text-slate-500"
                >{{ stagedAt }}</span>
                <span class="text-xs px-2 py-0.5 rounded-full bg-amber-100 dark:bg-amber-900/40 text-amber-700 dark:text-amber-400 font-medium">
                  {{ book.queuePosition }} of {{ book.totalQueued }}
                </span>
              </div>
            </div>
            <p class="mt-1 text-sm text-slate-500 dark:text-slate-400 font-mono break-all">
              {{ book.ircFilename }}
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
              <label class="text-xs text-slate-500 dark:text-slate-400 text-right">File</label>
              <input
                v-model="editFileName"
                type="text"
                placeholder="book.epub"
                class="w-full px-3 py-1.5 text-sm font-mono rounded-lg border border-slate-300 dark:border-slate-600 bg-white dark:bg-slate-800 text-slate-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
                @input="fileNameEdited = true"
              />
              <label class="text-xs text-slate-500 dark:text-slate-400 text-right self-start pt-2">Series</label>
              <div class="space-y-2">
                <input
                  v-model="editSeries"
                  type="text"
                  list="staged-series-suggestions"
                  placeholder="(none)"
                  class="w-full px-3 py-1.5 text-sm rounded-lg border border-slate-300 dark:border-slate-600 bg-white dark:bg-slate-800 text-slate-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
                <datalist id="staged-series-suggestions">
                  <option v-for="s in appStore.knownSeries" :key="s" :value="s" />
                </datalist>
                <div class="flex items-center gap-2">
                  <label class="text-xs text-slate-400 dark:text-slate-500 whitespace-nowrap">Sequence</label>
                  <input
                    v-model="editSeriesIndex"
                    type="text"
                    inputmode="decimal"
                    placeholder="0"
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

          <!-- Rewrite metadata toggle -->
          <div
            v-if="book.ircFilename.toLowerCase().endsWith('.epub')"
            class="flex items-start gap-3 p-3 rounded-xl border transition-colors"
            :class="rewriteMetadata
              ? 'border-blue-400 dark:border-blue-600 bg-blue-50/50 dark:bg-blue-950/20'
              : 'border-slate-200 dark:border-slate-700'"
          >
            <input
              id="staged-rewrite-toggle"
              type="checkbox"
              v-model="rewriteMetadata"
              class="mt-0.5 accent-blue-500 shrink-0"
            />
            <label for="staged-rewrite-toggle" class="cursor-pointer">
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
          <!-- Delete with inline confirmation -->
          <div class="flex items-center gap-2">
            <template v-if="confirmingDelete">
              <span class="text-xs text-red-500 dark:text-red-400">Delete this file?</span>
              <button
                @click="deleteStaged"
                class="px-3 py-1.5 text-xs font-medium rounded-lg bg-red-600 hover:bg-red-700 text-white transition-colors"
              >Yes, delete</button>
              <button
                @click="confirmingDelete = false"
                class="px-3 py-1.5 text-xs rounded-lg border border-slate-300 dark:border-slate-600 text-slate-600 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-800 transition-colors"
              >Cancel</button>
            </template>
            <button
              v-else
              @click="confirmingDelete = true"
              class="p-2 rounded-lg text-slate-400 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors"
              title="Delete staged file"
            >
              <Trash2 :size="16" />
            </button>
          </div>
          <div class="flex items-center gap-2">
            <button
              @click="saveLater"
              class="px-4 py-2 text-sm rounded-lg border border-amber-300 dark:border-amber-600 text-amber-700 dark:text-amber-400 hover:bg-amber-50 dark:hover:bg-amber-900/20 transition-colors"
            >
              Save Later
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
    </div>
  </Transition>
</template>

<style scoped>
.modal-enter-active,
.modal-leave-active {
  transition: opacity 0.2s ease;
}
.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}
</style>
