#!/bin/sh
# Post-process pipeline for openbooks downloads:
#   1. ebook-polish the downloaded file (subset fonts, tidy CSS, etc.)
#   2. audiobook-organizer (abo) --flat it into /books as Author/Title/
# openbooks invokes us as: abo-pipeline <file>
# Output goes to stdout (openbooks captures it) AND /books/.abo-pipeline.log.
set -u
FILE="$1"
BOOKS_DIR="${BOOKS_DIR:-/books}"
LOG="$BOOKS_DIR/.abo-pipeline.log"
LOG_PREFIX="[abo-pipeline]"

main() {
  echo "$LOG_PREFIX $(date -Iseconds) start: $FILE"
  if [ -z "$FILE" ] || [ ! -f "$FILE" ]; then
    echo "$LOG_PREFIX no file given: '$FILE'" >&2
    return 0
  fi

  # 1. Polish (non-fatal if it fails)
  if command -v ebook-polish >/dev/null 2>&1; then
    ebook-polish \
      --embed-fonts --subset-fonts --smarten-punctuation \
      --upgrade-book --remove-unused-css --compress-images \
      --add-soft-hyphens "$FILE" || echo "$LOG_PREFIX ebook-polish failed on $FILE" >&2
  fi

  # 2. Organize: stage the single file so abo --flat only touches this download
  STAGE=$(mktemp -d /tmp/abo-stage.XXXXXX) || return 1
  mv "$FILE" "$STAGE/" || return 1

  audiobook-organizer \
    --dir="$STAGE" \
    --out="$BOOKS_DIR" \
    --flat \
    --skip-errors || echo "$LOG_PREFIX organizer failed on $FILE" >&2

  # Return anything abo could not file back to the download dir so it isn't lost
  ORIG_DIR="$(dirname "$FILE")"
  for leftover in "$STAGE"/*; do
    [ -e "$leftover" ] && mv "$leftover" "$ORIG_DIR/" 2>/dev/null
  done
  rmdir "$STAGE" 2>/dev/null
  echo "$LOG_PREFIX $(date -Iseconds) done: $FILE"
  return 0
}

main "$@" 2>&1 | tee -a "$LOG"
exit 0
