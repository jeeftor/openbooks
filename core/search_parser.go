package core

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
)

// List of file extensions that I've encountered.
// Some of them aren't eBooks, but they were returned
// in previous search results.
var fileTypes = [...]string{
	"epub",
	"mobi",
	"azw3",
	"html",
	"rtf",
	"pdf",
	"cdr",
	"lit",
	"cbr",
	"doc",
	"htm",
	"jpg",
	"txt",
	"opf",
	"pdb",
	"m4b",
	"rar", // Compressed extensions should always be last 2 items
	"zip",
}

var (
	parenthesizedFormatPattern = regexp.MustCompile(`(?i)\s+\((epub|mobi|azw3|html|rtf|pdf|cdr|lit|cbr|doc|htm|jpg|txt|opf|pdb|m4b|rar|zip)\)\s+([0-9]+(?:\.[0-9]+)?\s*[kmgt]?i?b)\b`)
	languageMarkerPattern      = regexp.MustCompile(`(?i)\s+\[[a-z]{2,3}\]`)
)

// BookDetail contains the details of a single Book found on the IRC server
type BookDetail struct {
	Server string `json:"server"`
	Author string `json:"author"`
	Title  string `json:"title"`
	Format string `json:"format"`
	Size   string `json:"size"`
	Full   string `json:"full"`
}

type ParseError struct {
	Line  string `json:"line"`
	Error error  `json:"error"`
}

func (p *ParseError) MarshalJSON() ([]byte, error) {
	item := struct {
		Line  string `json:"line"`
		Error string `json:"error"`
	}{
		Line:  p.Line,
		Error: p.Error.Error(),
	}
	return json.Marshal(item)
}

func (p ParseError) String() string {
	return fmt.Sprintf("Error: %s. Line: %s.", p.Error, p.Line)
}

// ParseSearchFile converts a single search file into an array of BookDetail
func ParseSearchFile(filePath string) ([]BookDetail, []ParseError, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	books, errs := ParseSearchV2(file)
	return books, errs, nil
}


func ParseSearchV2(reader io.Reader) ([]BookDetail, []ParseError) {
	books := make([]BookDetail, 0)
	parseErrors := make([]ParseError, 0)

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "!") {
			dat, err := parseLineV2(line)
			if err != nil {
				parseErrors = append(parseErrors, ParseError{Line: line, Error: err})
			} else {
				// Correct swapped author/title fields. IRC data sometimes
				// uses "Title - Author" format instead of "Author - Title".
				// Only swap when there's a strong signal — see detectAuthorTitleSwap.
				if detectAuthorTitleSwap(dat.Author, dat.Title) {
					dat.Author, dat.Title = dat.Title, dat.Author
				}
				books = append(books, dat)
			}
		}
	}

	sort.Slice(books, func(i, j int) bool { return books[i].Server < books[j].Server })

	return books, parseErrors
}

// detectAuthorTitleSwap returns true if the author and title fields appear to
// be swapped (IRC data sometimes uses "Title - Author" format instead of
// "Author - Title"). The heuristic is deliberately conservative — it only
// recommends a swap when there's a strong signal, leaving ambiguous cases
// as-is to avoid introducing new errors.
func detectAuthorTitleSwap(author, title string) bool {
	author = strings.TrimSpace(author)
	title = strings.TrimSpace(title)
	if author == "" || title == "" {
		return false
	}

	// 1. "et al" in title field — clearly an author attribution.
	if endsWithEtAl(title) && !endsWithEtAl(author) {
		return true
	}

	// 2. "Last, First" comma pattern in title but not author.
	if hasLastNameCommaPattern(title) && !hasLastNameCommaPattern(author) {
		return true
	}

	// 3. Title starts with initials pattern (e.g., "J. R. R. Tolkien")
	//    and author has no initials or comma pattern.
	if hasInitialsPattern(title) && !hasInitialsPattern(author) && !hasLastNameCommaPattern(author) {
		return true
	}

	// 4. Author starts with a definite article ("The/A/An") and title
	//    doesn't and title looks like a person's name.
	if startsWithArticle(author) && !startsWithArticle(title) && isNameLike(title) {
		return true
	}

	// 5. Author ends with inverted article (", The/A/An") and title
	//    looks like a person's name.
	if endsWithInvertedArticle(author) && isNameLike(title) {
		return true
	}

	// 6. Author is a single word and title is 2-4 words and looks like
	//    a name. Single-word authors are very rare in ebook IRC data.
	if wordCount(author) == 1 && wordCount(title) >= 2 && wordCount(title) <= 4 && isNameLike(title) {
		return true
	}

	// 7. Author has >3x as many words as title and title looks like a name.
	if wordCount(author) > wordCount(title)*3 && wordCount(title) >= 2 && wordCount(title) <= 4 && isNameLike(title) {
		return true
	}

	return false
}

// startsWithArticle returns true if s starts with "The ", "A ", or "An ".
func startsWithArticle(s string) bool {
	lower := strings.ToLower(strings.TrimSpace(s))
	return strings.HasPrefix(lower, "the ") ||
		strings.HasPrefix(lower, "a ") ||
		strings.HasPrefix(lower, "an ")
}

// endsWithInvertedArticle returns true if s ends with ", The", ", A", or ", An".
func endsWithInvertedArticle(s string) bool {
	lower := strings.ToLower(strings.TrimSpace(s))
	return strings.HasSuffix(lower, ", the") ||
		strings.HasSuffix(lower, ", a") ||
		strings.HasSuffix(lower, ", an")
}

// endsWithEtAl returns true if s ends with "et al" or "et al.".
func endsWithEtAl(s string) bool {
	lower := strings.ToLower(strings.TrimSpace(s))
	return strings.HasSuffix(lower, " et al") || strings.HasSuffix(lower, " et al.")
}

// hasInitialsPattern returns true if s starts with one or more single-letter
// words (optionally followed by periods), like "J. R. R. Tolkien" or
// "J R R Tolkien".
func hasInitialsPattern(s string) bool {
	fields := strings.Fields(s)
	if len(fields) < 2 {
		return false
	}
	for _, f := range fields {
		cleaned := strings.TrimSuffix(f, ".")
		if len(cleaned) == 1 && ((cleaned[0] >= 'a' && cleaned[0] <= 'z') || (cleaned[0] >= 'A' && cleaned[0] <= 'Z')) {
			continue // initial
		}
		// First non-initial word — check we saw at least one initial before it.
		break
	}
	// Check if the first field is a single letter (with optional period).
	first := strings.TrimSuffix(fields[0], ".")
	return len(first) == 1 && ((first[0] >= 'a' && first[0] <= 'z') || (first[0] >= 'A' && first[0] <= 'Z'))
}

// hasLastNameCommaPattern returns true if s has a "Last, First" pattern —
// a comma after the first word, with non-empty content after the comma.
// e.g., "Tolkien, J. R. R." or "Brown, Dan".
func hasLastNameCommaPattern(s string) bool {
	s = strings.TrimSpace(s)
	idx := strings.Index(s, ",")
	if idx <= 0 {
		return false
	}
	before := strings.TrimSpace(s[:idx])
	after := strings.TrimSpace(s[idx+1:])
	if before == "" || after == "" {
		return false
	}
	// "Before" should be a single word (the last name).
	beforeWords := strings.Fields(before)
	return len(beforeWords) == 1
}

// isNameLike returns true if s looks like a person's name rather than a
// book title: 2-5 words, no leading/trailing articles, no colons or
// semicolons (subtitle markers), no underscores (IRC filename artifact).
func isNameLike(s string) bool {
	fields := strings.Fields(s)
	if len(fields) < 2 || len(fields) > 5 {
		return false
	}
	if startsWithArticle(s) || endsWithInvertedArticle(s) {
		return false
	}
	if strings.ContainsAny(s, ":;_") {
		return false
	}
	return true
}

// wordCount returns the number of whitespace-separated words in s.
func wordCount(s string) int {
	return len(strings.Fields(s))
}

func parseLineV2(line string) (BookDetail, error) {
	getServer := func(line string) (string, error) {
		if line[0] != '!' {
			return "", errors.New("result lines must start with '!'")
		}

		firstSpace := strings.Index(line, " ")
		if firstSpace == -1 {
			return "", errors.New("unable parse server name")
		}

		return line[1:firstSpace], nil
	}

	getAuthor := func(line string) (string, error) {
		firstSpace := strings.Index(line, " ")
		dashChar, _ := findAuthorTitleSeparator(line)
		if dashChar == -1 {
			return "", errors.New("unable to parse author")
		}
		author := line[firstSpace+len(" ") : dashChar]

		// Handles case with weird author characters %\w% ("%F77FE9FF1CCD% Michael Haag")
		if strings.Contains(author, "%") {
			split := strings.SplitAfterN(author, " ", 2)
			if len(split) == 2 {
				return strings.TrimSpace(split[1]), nil
			}
		}

		// Handles "HASH | Author Name" format used by some bots
		// e.g. "ab94ae567320 | Brandon Sanderson"
		if pipeIdx := strings.Index(author, " | "); pipeIdx != -1 {
			return strings.TrimSpace(author[pipeIdx+3:]), nil
		}

		return author, nil
	}

	getTitle := func(line string) (string, string, int) {
		title := ""
		fileFormat := ""
		endIndex := -1
		// Get the Title
		lowerLine := strings.ToLower(line)
		for _, ext := range fileTypes { //Loop through each possible file extension we've got on record
			endTitle := strings.Index(lowerLine, "."+ext) // check if it contains our extension
			if endTitle == -1 {
				continue
			}
			fileFormat = ext
			if ext == "rar" || ext == "zip" { // If the extension is .rar or .zip the actual format is contained in ()
				for _, ext2 := range fileTypes[:len(fileTypes)-2] { // Range over the eBook formats (exclude archives)
					if strings.Contains(strings.ToLower(line[:endTitle]), ext2) {
						fileFormat = ext2
					}
				}
			}
			startIndex, sepLen := findAuthorTitleSeparator(line)
			if startIndex == -1 {
				continue
			}
			title = strings.TrimSpace(line[startIndex+sepLen : endTitle])
			endIndex = endTitle
		}

		return title, fileFormat, endIndex
	}

	getSize := func(line string) (string, int) {
		const delimiter = " ::INFO:: "
		infoIndex := strings.LastIndex(line, delimiter)

		if infoIndex != -1 {
			// Handle cases when there is additional info after the file size (ex ::HASH:: )
			parts := strings.Split(line[infoIndex+len(delimiter):], " ")
			return parts[0], infoIndex
		}

		return "N/A", len(line)
	}

	server, err := getServer(line)
	if err != nil {
		return BookDetail{}, err
	}

	if book, ok := parseParenthesizedFormatLine(server, line); ok {
		return book, nil
	}

	if book, ok := parseFilenameOnlyLine(server, line, getSize); ok {
		return book, nil
	}

	author, err := getAuthor(line)
	if err != nil {
		return BookDetail{}, err
	}

	title, format, titleIndex := getTitle(line)
	if titleIndex == -1 {
		return BookDetail{}, errors.New("unable to parse title")
	}

	size, endIndex := getSize(line)

	return BookDetail{
		Server: server,
		Author: author,
		Title:  title,
		Format: format,
		Size:   size,
		Full:   strings.TrimSpace(line[:endIndex]),
	}, nil
}

func findAuthorTitleSeparator(line string) (int, int) {
	if idx := strings.Index(line, " - "); idx != -1 {
		return idx, len(" - ")
	}
	if idx := strings.Index(line, " -"); idx != -1 {
		return idx, len(" -")
	}
	return -1, 0
}

func parseParenthesizedFormatLine(server, line string) (BookDetail, bool) {
	firstSpace := strings.Index(line, " ")
	if firstSpace == -1 {
		return BookDetail{}, false
	}

	body := strings.TrimSpace(line[firstSpace+1:])
	matches := parenthesizedFormatPattern.FindAllStringSubmatchIndex(body, -1)
	if len(matches) == 0 {
		return BookDetail{}, false
	}
	match := matches[len(matches)-1]

	prefix := strings.TrimSpace(body[:match[0]])
	parts := strings.Split(prefix, " - ")
	if len(parts) < 2 {
		return BookDetail{}, false
	}

	authorIndex := 0
	if len(parts) >= 3 && looksLikeResultToken(parts[0]) {
		authorIndex = 1
	}
	if len(parts[authorIndex:]) < 2 {
		return BookDetail{}, false
	}

	author := strings.Trim(strings.TrimSpace(parts[authorIndex]), " ;")
	title := strings.Join(parts[authorIndex+1:], " - ")
	title = strings.TrimSpace(languageMarkerPattern.ReplaceAllString(title, ""))
	if author == "" || title == "" {
		return BookDetail{}, false
	}

	format := strings.ToLower(body[match[2]:match[3]])
	size := strings.TrimSpace(body[match[4]:match[5]])
	fullEnd := firstSpace + 1 + match[1]

	return BookDetail{
		Server: server,
		Author: author,
		Title:  title,
		Format: format,
		Size:   size,
		Full:   strings.TrimSpace(line[:fullEnd]),
	}, true
}

func parseFilenameOnlyLine(server, line string, getSize func(string) (string, int)) (BookDetail, bool) {
	if sepIdx, _ := findAuthorTitleSeparator(line); sepIdx != -1 {
		return BookDetail{}, false
	}

	start := strings.Index(line, " ")
	if start == -1 {
		return BookDetail{}, false
	}
	start++
	if pipeIdx := strings.Index(line[start:], " | "); pipeIdx != -1 {
		start += pipeIdx + len(" | ")
	}

	lowerLine := strings.ToLower(line)
	for _, ext := range fileTypes {
		endTitle := strings.Index(lowerLine[start:], "."+ext)
		if endTitle == -1 {
			continue
		}
		endTitle += start
		title := strings.TrimSpace(line[start:endTitle])
		if title == "" {
			return BookDetail{}, false
		}
		size, endIndex := getSize(line)
		return BookDetail{
			Server: server,
			Title:  title,
			Format: ext,
			Size:   size,
			Full:   strings.TrimSpace(line[:endIndex]),
		}, true
	}

	return BookDetail{}, false
}

func looksLikeResultToken(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) < 12 || strings.Contains(value, " ") {
		return false
	}

	hasDigit := false
	for _, r := range value {
		switch {
		case r >= '0' && r <= '9':
			hasDigit = true
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case r == '+' || r == '/':
		default:
			return false
		}
	}

	return hasDigit
}
