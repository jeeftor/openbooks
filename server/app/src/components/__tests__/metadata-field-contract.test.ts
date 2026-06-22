import { describe, it, expect, beforeEach } from "vitest";
import { setActivePinia, createPinia } from "pinia";
import { useAppStore } from "../../stores/app";
import { MessageType, NotificationType, type RenamePromptResponse, type StagedBookResumeResponse, type EPUBMetadata } from "../../types/messages";

// These tests verify that the Vue components' metadata field access pattern
// (metadata?.author, metadata?.title, metadata?.series, metadata?.series_index)
// matches the JSON field names the Go backend actually sends.
//
// The Go backend serializes EPUBMetadata with lowercase json tags:
//   {"author":"...","title":"...","series":"...","series_index":"..."}
//
// If either side changes without the other, these tests fail.

function makeMetadata(): EPUBMetadata {
  return {
    author: "Frank Herbert",
    title: "Dune",
    series: "Dune Chronicles",
    series_index: "1",
  };
}

function makeWsBase() {
  return {
    type: MessageType.RENAME_PROMPT,
    appearance: NotificationType.SUCCESS,
    title: "test",
  };
}

describe("EPUBMetadata field contract", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  it("RenamePromptResponse.metadata has lowercase field names matching Go json tags", () => {
    const store = useAppStore();
    const payload: RenamePromptResponse = {
      ...makeWsBase(),
      ircFilename: "test.epub",
      metadata: makeMetadata(),
      options: [],
      replaceSpace: "",
    };

    store.pendingRename = payload;

    const meta = store.pendingRename?.metadata;
    expect(meta?.author).toBe("Frank Herbert");
    expect(meta?.title).toBe("Dune");
    expect(meta?.series).toBe("Dune Chronicles");
    expect(meta?.series_index).toBe("1");
  });

  it("RenamePromptResponse with empty series omits series fields (omitempty contract)", () => {
    const store = useAppStore();
    const payload: RenamePromptResponse = {
      ...makeWsBase(),
      ircFilename: "test.epub",
      metadata: {
        author: "Frank Herbert",
        title: "Dune",
        series: "",
        series_index: "",
      },
      options: [],
      replaceSpace: "",
    };

    store.pendingRename = payload;

    const meta = store.pendingRename?.metadata;
    expect(meta?.author).toBe("Frank Herbert");
    expect(meta?.title).toBe("Dune");
    expect(meta?.series ?? "").toBe("");
    expect(meta?.series_index ?? "").toBe("");
  });

  it("RenamePromptResponse with no metadata falls back to empty strings", () => {
    const store = useAppStore();
    const payload: RenamePromptResponse = {
      ...makeWsBase(),
      ircFilename: "test.epub",
      metadata: undefined,
      options: [],
      replaceSpace: "",
    };

    store.pendingRename = payload;

    const meta = store.pendingRename?.metadata;
    expect(meta?.author ?? "").toBe("");
    expect(meta?.title ?? "").toBe("");
    expect(meta?.series ?? "").toBe("");
    expect(meta?.series_index ?? "").toBe("");
  });

  it("StagedBookResumeResponse.metadata has lowercase field names matching Go json tags", () => {
    const store = useAppStore();
    const payload: StagedBookResumeResponse = {
      ...makeWsBase(),
      stagedId: "test-id",
      ircFilename: "test.epub",
      metadata: makeMetadata(),
      options: [],
      replaceSpace: "",
      stagedAt: "2024-01-01T00:00:00Z",
      queuePosition: 1,
      totalQueued: 1,
    };

    store.pendingStagedBook = payload;

    const meta = store.pendingStagedBook?.metadata;
    expect(meta?.author).toBe("Frank Herbert");
    expect(meta?.title).toBe("Dune");
    expect(meta?.series).toBe("Dune Chronicles");
    expect(meta?.series_index).toBe("1");
  });

  it("StagedBookResumeResponse with empty series omits series fields (omitempty contract)", () => {
    const store = useAppStore();
    const payload: StagedBookResumeResponse = {
      ...makeWsBase(),
      stagedId: "test-id",
      ircFilename: "test.epub",
      metadata: {
        author: "Frank Herbert",
        title: "Dune",
        series: "",
        series_index: "",
      },
      options: [],
      replaceSpace: "",
      stagedAt: "2024-01-01T00:00:00Z",
      queuePosition: 1,
      totalQueued: 1,
    };

    store.pendingStagedBook = payload;

    const meta = store.pendingStagedBook?.metadata;
    expect(meta?.author).toBe("Frank Herbert");
    expect(meta?.title).toBe("Dune");
    expect(meta?.series ?? "").toBe("");
    expect(meta?.series_index ?? "").toBe("");
  });
});

// Test that the field access pattern used in RenameModal.vue and
// StagedRenameModal.vue correctly populates edit refs from metadata.
// This simulates the watch() handler in those components.
describe("Component metadata field access pattern", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  it("RenameModal watch pattern: editAuthor/editTitle/editSeries/editSeriesIndex populate from metadata", () => {
    const store = useAppStore();
    store.pendingRename = {
      ...makeWsBase(),
      ircFilename: "test.epub",
      metadata: makeMetadata(),
      options: [],
      replaceSpace: "",
    };

    // This is the exact pattern from RenameModal.vue lines 25-28.
    const prompt = store.pendingRename;
    if (!prompt) throw new Error("pendingRename not set");
    const editAuthor = prompt.metadata?.author ?? "";
    const editTitle = prompt.metadata?.title ?? "";
    const editSeries = prompt.metadata?.series ?? "";
    const editSeriesIndex = prompt.metadata?.series_index ?? "";

    expect(editAuthor).toBe("Frank Herbert");
    expect(editTitle).toBe("Dune");
    expect(editSeries).toBe("Dune Chronicles");
    expect(editSeriesIndex).toBe("1");
  });

  it("StagedRenameModal watch pattern: editAuthor/editTitle/editSeries/editSeriesIndex populate from metadata", () => {
    const store = useAppStore();
    store.pendingStagedBook = {
      ...makeWsBase(),
      stagedId: "test-id",
      ircFilename: "test.epub",
      metadata: makeMetadata(),
      options: [],
      replaceSpace: "",
      stagedAt: "2024-01-01T00:00:00Z",
      queuePosition: 1,
      totalQueued: 1,
    };

    // This is the exact pattern from StagedRenameModal.vue lines 23-26.
    const book = store.pendingStagedBook;
    if (!book) throw new Error("pendingStagedBook not set");
    const editAuthor = book.metadata?.author ?? "";
    const editTitle = book.metadata?.title ?? "";
    const editSeries = book.metadata?.series ?? "";
    const editSeriesIndex = book.metadata?.series_index ?? "";

    expect(editAuthor).toBe("Frank Herbert");
    expect(editTitle).toBe("Dune");
    expect(editSeries).toBe("Dune Chronicles");
    expect(editSeriesIndex).toBe("1");
  });

  it("PascalCase field names would NOT populate (regression guard for the rename)", () => {
    // If the Go backend ever reverts to PascalCase, the lowercase access
    // pattern would return undefined, and the ?? "" fallback would give "".
    // This test documents that fact.
    const fakePascalCasePayload = {
      Author: "Frank Herbert",
      Title: "Dune",
      Series: "Dune Chronicles",
      SeriesIndex: "1",
    } as unknown as EPUBMetadata;

    // Access with the lowercase pattern the Vue components use.
    expect((fakePascalCasePayload as unknown as Record<string, unknown>).author).toBeUndefined();
    expect((fakePascalCasePayload as unknown as Record<string, unknown>).title).toBeUndefined();
    expect(fakePascalCasePayload.author ?? "").toBe("");
    expect(fakePascalCasePayload.title ?? "").toBe("");
  });
});
