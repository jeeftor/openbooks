import type { Book } from "../types/messages";

export interface BookGroup {
  key: string;
  title: string;
  author: string;
  series: string;
  seriesIndex: string;
  files: Book[];
  newest: number;
}

function normalize(value: string): string {
  return value
    .normalize("NFKD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, " ")
    .trim();
}

export function groupLibraryBooks(books: Book[], sortMode: "newest" | "alpha"): BookGroup[] {
  const groups: BookGroup[] = [];
  const byMetadata = new Map<string, BookGroup>();
  const byDirectory = new Map<string, BookGroup>();

  for (const book of books) {
    const key = `${normalize(book.author)}\u0000${normalize(book.title)}`;
    const slash = book.path.lastIndexOf("/");
    const directory = slash > 0 ? normalize(book.path.slice(0, slash)) : "";
    const modified = new Date(book.time).getTime();
    const existing = byMetadata.get(key) ?? (directory ? byDirectory.get(directory) : undefined);
    if (existing) {
      existing.files.push(book);
      existing.newest = Math.max(existing.newest, modified);
      if (book.title.length < existing.title.length) existing.title = book.title;
      if (!existing.series && book.series) existing.series = book.series;
      if (!existing.seriesIndex && book.seriesIndex) existing.seriesIndex = book.seriesIndex;
      byMetadata.set(key, existing);
      continue;
    }
    const group: BookGroup = {
      key,
      title: book.title,
      author: book.author,
      series: book.series ?? "",
      seriesIndex: book.seriesIndex ?? "",
      files: [book],
      newest: modified
    };
    groups.push(group);
    byMetadata.set(key, group);
    if (directory) byDirectory.set(directory, group);
  }

  const result = [...groups];
  if (sortMode === "alpha") {
    return result.sort((a, b) => a.title.localeCompare(b.title) || a.author.localeCompare(b.author));
  }
  return result.sort((a, b) => b.newest - a.newest);
}
