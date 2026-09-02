import { describe, expect, it } from "vitest";
import type { Book } from "../types/messages";
import { groupLibraryBooks } from "./library";

function book(overrides: Partial<Book>): Book {
  return {
    name: "book.epub",
    path: "Author/Title/book.epub",
    downloadLink: "library/Author/Title/book.epub",
    time: "2026-09-01T12:00:00Z",
    format: "epub",
    author: "Author",
    title: "Title",
    ...overrides
  };
}

describe("groupLibraryBooks", () => {
  it("groups metadata-equivalent files and retains their formats", () => {
    const groups = groupLibraryBooks([
      book({ name: "title.epub" }),
      book({
        name: "title.mobi",
        path: "Author/Title/title.mobi",
        downloadLink: "library/Author/Title/title.mobi",
        format: "mobi",
        author: "Áuthor",
        title: "Series Name - Title",
        series: "Series",
        seriesIndex: "2"
      })
    ], "alpha");

    expect(groups).toHaveLength(1);
    expect(groups[0].files.map((file) => file.format)).toEqual(["epub", "mobi"]);
    expect(groups[0].series).toBe("Series");
    expect(groups[0].seriesIndex).toBe("2");
  });

  it("sorts logical books by newest file", () => {
    const groups = groupLibraryBooks([
      book({ title: "Older", path: "Author/Older/book.epub", time: "2026-08-01T12:00:00Z" }),
      book({ title: "Newer", path: "Author/Newer/book.epub", time: "2026-09-01T12:00:00Z" })
    ], "newest");

    expect(groups.map((group) => group.title)).toEqual(["Newer", "Older"]);
  });
});
