#!/bin/sh
# Post-process pipeline for openbooks downloads:
#   1. ebook-polish the downloaded file (subset fonts, tidy CSS, etc.)
#   2. audiobook-organizer (abo) --flat it into /books as Author/Title/
# openbooks invokes us as: abo-pipeline <file>
set -u
FILE="$1"
BOOKS_DIR="${BOOKS_DIR:-/books}"
LOG_PREFIX="[abo-pipeline]"

if [ -z "$FILE" ] || [ ! -f "$FILE" ]; then
  echo "$LOG_PREFIX no file given: '$FILE'" >&2
  exit 0
fi

# 1. Polish (non-fatal if it fails)
if command -v ebook-polish >/dev/null 2>&1; then
  ebook-polish \
    --embed-fonts --subset-fonts --smarten-punctuation \
    --upgrade-book --remove-unused-css --compress-images \
    --add-soft-hyphens "$FILE" || echo "$LOG_PREFIX ebook-polish failed on $FILE" >&2
fi

# 2. Organize: stage the single file so abo --flat only touches this download
STAGE=$(mktemp -d /tmp/abo-stage.XXXXXX) || exit 1
mv "$FILE" "$STAGE/" || exit 1

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
exit 0
