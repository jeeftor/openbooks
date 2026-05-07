package server

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/evan-buss/openbooks/core"
	"github.com/evan-buss/openbooks/dcc"
)

func fileSizeMB(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return "? MB"
	}
	mb := float64(info.Size()) / 1024 / 1024
	return fmt.Sprintf("%.1f MB", mb)
}

func (server *server) NewIrcEventHandler(client *Client) core.EventHandler {
	handler := core.EventHandler{}
	handler[core.SearchResult] = client.searchResultHandler(server.config.DownloadDir, server.logBuf)
	handler[core.BookResult] = client.bookResultHandler(*server.config, server.logBuf)
	handler[core.NoResults] = client.noResultsHandler
	handler[core.BadServer] = client.badServerHandler
	handler[core.SearchAccepted] = client.searchAcceptedHandler
	handler[core.MatchesFound] = client.matchesFoundHandler
	handler[core.Ping] = client.pingHandler
	handler[core.ServerList] = client.userListHandler(server.repository)
	handler[core.Version] = client.versionHandler(server.config.UserAgent)
	return handler
}

// searchResultHandler downloads from DCC server, parses data, and sends data to client
func (c *Client) searchResultHandler(downloadDir string, lb *logBuffer) core.HandlerFunc {
	return func(text string) {
		extractedPath, err := core.DownloadExtractDCCString(downloadDir, text, nil)
		if err != nil {
			c.log.Println(err)
			c.send <- newErrorResponse("Error when downloading search results.")
			return
		}

		bookResults, parseErrors, err := core.ParseSearchFile(extractedPath)
		if err != nil {
			c.log.Println(err)
			c.send <- newErrorResponse("Error when parsing search results.")
			return
		}
		rawResults, err := os.ReadFile(extractedPath)
		if err != nil {
			c.log.Printf("Error reading raw search results file: %v", err)
		}

		if len(bookResults) == 0 && len(parseErrors) == 0 {
			c.noResultsHandler(text)
			return
		}

		// Output all errors so parser can be improved over time
		if len(parseErrors) > 0 {
			c.log.Printf("%d Search Result Parsing Errors\n", len(parseErrors))
			for _, err := range parseErrors {
				c.log.Println(err)
			}
		}

		c.log.Printf("Sending %d search results.\n", len(bookResults))
		lb.info(fmt.Sprintf("🔍 Search results: %d found, %d unparseable", len(bookResults), len(parseErrors)))
		c.send <- newSearchResponse(bookResults, parseErrors, string(rawResults))

		err = os.Remove(extractedPath)
		if err != nil {
			c.log.Printf("Error deleting search results file: %v", err)
		}
	}
}

// bookResultHandler implements the staging→post-process→prompt→confirm→move flow.
//
//  1. Download the book to the hidden .staging directory.
//  2. Run the post-processor (e.g. ebook-polish) on the staged file so metadata is clean.
//  3. Read EPUB metadata from the now-polished file.
//  4. Send RENAME_PROMPT to the frontend with generated naming options.
//  5. Block until the user confirms (or a 30-min timeout elapses).
//  6. Move from staging to the confirmed final path.
//  7. Optionally rewrite EPUB internal metadata.
//  8. Signal the download queue so the next book can begin.
func (c *Client) bookResultHandler(config Config, lb *logBuffer) core.HandlerFunc {
	return func(text string) {
		dir := config.DownloadDir

		// Ensure the staging directory exists before downloading.
		if err := ensureStagingDir(dir); err != nil {
			c.log.Printf("Failed to create staging dir: %v", err)
			lb.error("Failed to create staging directory.")
			c.send <- newErrorResponse("Internal error: could not create staging directory.")
			signalDone(c)
			return
		}
		stage := stagingDir(dir)

		// DCC offer received — clear the "waiting for bot" UI state and signal transfer start.
		safeSend(c, newDownloadWaitingClear())
		safeSend(c, newDownloadStartedResponse())

		// Determine the filename for the initial log entry and session group.
		var ircFilenamePreview string
		if d, err := dcc.ParseString(text); err == nil {
			ircFilenamePreview = d.Filename
		}

		// Create a session so all entries for this download are grouped together.
		// Group is set to the IRC filename; falls back to a timestamp if unavailable.
		group := ircFilenamePreview
		if group == "" {
			group = fmt.Sprintf("dl-%d", time.Now().UnixMilli())
		}
		sess := lb.session(group)
		sess.info(fmt.Sprintf("⬇️  Downloading: %s", ircFilenamePreview))

		// 1. Download to staging.
		extractedPath, err := core.DownloadExtractDCCString(stage, text, nil)
		if err != nil {
			c.log.Println(err)
			sess.error(fmt.Sprintf("Download failed: %v", err))
			c.send <- newErrorResponse("Error when downloading book.")
			signalDone(c)
			return
		}

		size := fileSizeMB(extractedPath)
		ircFilename := filepath.Base(extractedPath)
		sess.infoDetail(
			fmt.Sprintf("📥 Downloaded: %s (%s)", ircFilename, size),
			fmt.Sprintf("File: %s\nSize: %s\nStaged at: %s", ircFilename, size, extractedPath),
		)

		var stagedOriginalPath string
		if config.DevMode {
			stagedOriginalPath = originalCopyPath(extractedPath)
			if err := copyFile(extractedPath, stagedOriginalPath); err != nil {
				sess.warn(fmt.Sprintf("Could not preserve original download: %v", err))
				c.log.Printf("Failed to preserve original download: %v", err)
				stagedOriginalPath = ""
			}
		}

		// 2. Run post-processor on the staged file first so metadata is clean.
		if len(config.PostProcessCmd) > 0 {
			safeSend(c, newPostProcessStartedResponse())
		}
		runPostProcess(config.PostProcessCmd, extractedPath, sess)

		// 3. Read EPUB metadata and cover (from the polished file).
		var meta *core.EPUBMetadata
		var coverBase64, coverMime string
		if strings.EqualFold(filepath.Ext(extractedPath), ".epub") {
			if m, err := core.ReadEPUBMetadata(extractedPath); err == nil {
				meta = m
			} else {
				c.log.Printf("Metadata read failed for %s: %v", ircFilename, err)
			}
			if imgBytes, mime, err := core.ExtractCoverImage(extractedPath); err == nil && imgBytes != nil {
				coverBase64 = base64.StdEncoding.EncodeToString(imgBytes)
				coverMime = mime
			}
		}

		// 4. Build options and send prompt to the UI.
		options := buildRenameOptions(ircFilename, meta, config.ReplaceSpace)
		safeSend(c, RenamePromptResponse{
			StatusResponse: StatusResponse{
				MessageType:      RENAME_PROMPT,
				NotificationType: NOTIFY,
				Title:            "Book downloaded — how would you like to save it?",
			},
			IRCFilename:  ircFilename,
			Metadata:     meta,
			Options:      options,
			ReplaceSpace: config.ReplaceSpace,
			CoverBase64:  coverBase64,
			CoverMime:    coverMime,
		})

		// 5. Wait for the user's rename decision.
		var choice RenameChoice
		select {
		case choice = <-c.renameConfirm:
		case <-time.After(30 * time.Minute):
			sess.warn(fmt.Sprintf("Rename timed out — keeping IRC filename: %s", ircFilename))
			choice = RenameChoice{OptionID: "keep"}
		case <-c.ctx.Done():
			// Client disconnected — clean up the staged file and exit.
			os.Remove(extractedPath)
			if stagedOriginalPath != "" {
				os.Remove(stagedOriginalPath)
			}
			return
		}

		// 6. Resolve option label, move from staging to final path.
		optionLabel := choice.OptionID
		for _, opt := range options {
			if opt.ID == choice.OptionID {
				optionLabel = opt.Label
				break
			}
		}

		finalPath := resolveFinalPath(dir, choice, ircFilename, meta, config.ReplaceSpace)

		if err := moveFile(extractedPath, finalPath); err != nil {
			c.log.Printf("Move failed: %v", err)
			sess.error(fmt.Sprintf("Failed to move file: %v", err))
			finalPath = extractedPath
		}
		if stagedOriginalPath != "" {
			originalFinalPath := originalCopyPath(finalPath)
			if err := moveFile(stagedOriginalPath, originalFinalPath); err != nil {
				c.log.Printf("Move original copy failed: %v", err)
				sess.warn(fmt.Sprintf("Failed to save original copy: %v", err))
			} else {
				relOrig, _ := filepath.Rel(dir, originalFinalPath)
				sess.infoDetail(
					fmt.Sprintf("🧪 Original preserved: %s", filepath.ToSlash(relOrig)),
					fmt.Sprintf("Path: %s", originalFinalPath),
				)
			}
		}

		// 7. Optionally rewrite the EPUB's internal OPF metadata.
		if choice.RewriteMetadata && strings.EqualFold(filepath.Ext(finalPath), ".epub") {
			if err := RewriteEPUBMetadata(finalPath, choice.Title, choice.Author, choice.Series, choice.SeriesIndex); err != nil {
				sess.warn(fmt.Sprintf("Metadata rewrite failed: %v", err))
				c.log.Printf("RewriteEPUBMetadata: %v", err)
			} else {
				sess.infoDetail("✏️  Metadata rewritten",
					fmt.Sprintf("Author: %s\nTitle: %s\nSeries: %s\nBook #: %s",
						choice.Author, choice.Title, choice.Series, choice.SeriesIndex))
			}
		}

		// 8. Single combined "saved" log entry (merges "saving as" + path).
		rel, _ := filepath.Rel(dir, finalPath)
		relSlash := filepath.ToSlash(rel)
		savedDetail := fmt.Sprintf("Option: %s\nAuthor: %s\nTitle: %s\nSeries: %s\nBook #: %s\nPath: %s",
			optionLabel, choice.Author, choice.Title, choice.Series, choice.SeriesIndex, finalPath)
		sess.infoDetail(fmt.Sprintf("✅ Saved [%s]: %s", optionLabel, relSlash), savedDetail)

		c.log.Printf("Book saved to: %s\n", finalPath)
		safeSend(c, newDownloadResponse(finalPath, dir))
		signalDone(c)
	}
}

// signalDone unblocks the download queue so the next job can start.
func signalDone(c *Client) {
	select {
	case c.downloadDone <- struct{}{}:
	default:
	}
}

// noResultsHandler is called when the server returns that nothing was found for the query
func (c *Client) noResultsHandler(_ string) {
	c.send <- newErrorResponse("No results found for the query.")
}

// badServerHandler is called when the requested download fails because the server is not available
func (c *Client) badServerHandler(_ string) {
	c.send <- newErrorResponse("Server is not available. Try another one.")
}

// searchAcceptedHandler is called when the user's query is accepted into the search queue
func (c *Client) searchAcceptedHandler(_ string) {
	c.send <- newStatusResponse(NOTIFY, "Search accepted into the queue.")
}

// matchesFoundHandler is called when the server finds matches for the user's query
func (c *Client) matchesFoundHandler(num string) {
	c.send <- newStatusResponse(NOTIFY, fmt.Sprintf("Found %s results for your query.", num))
}

func (c *Client) pingHandler(serverUrl string) {
	c.irc.Pong(serverUrl)
}

func (c *Client) versionHandler(version string) core.HandlerFunc {
	return func(line string) {
		c.log.Printf("Sending CTCP version response: %s", line)
		core.SendVersionInfo(c.irc, line, version)
	}
}

func (c *Client) userListHandler(repo *Repository) core.HandlerFunc {
	return func(text string) {
		repo.servers = core.ParseServers(text)
	}
}
