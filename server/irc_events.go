package server

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jeeftor/openbooks/core"
	"github.com/jeeftor/openbooks/dcc"
	"github.com/jeeftor/openbooks/staging"
	"github.com/google/uuid"
)

func fileSizeMB(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return "? MB"
	}
	return formatBytes(info.Size())
}

// NewIrcEventHandler builds the event handler map for a session's IRC connection.
func (server *server) NewIrcEventHandler(sess *session) core.EventHandler {
	handler := core.EventHandler{}
	handler[core.SearchResult] = sess.searchResultHandler(server.config.DownloadDir, server.logBuf, server)
	handler[core.BookResult] = sess.bookResultHandler(*server.config, server.logBuf, server.stagedBooks, server.seriesRegistry, server)
	handler[core.NoResults] = sess.noResultsHandler()
	handler[core.BadServer] = sess.badServerHandler()
	handler[core.SearchAccepted] = sess.searchAcceptedHandler()
	handler[core.MatchesFound] = sess.matchesFoundHandler()
	handler[core.Ping] = sess.pingHandler()
	handler[core.ServerList] = sess.userListHandler(server)
	handler[core.Version] = sess.versionHandler(server.config.UserAgent)
	return handler
}

// searchResultHandler downloads from DCC server, parses data, and sends data to client.
func (sess *session) searchResultHandler(downloadDir string, lb *logBuffer, srv *server) core.HandlerFunc {
	return func(text string) {
		extractedPath, err := core.DownloadExtractDCCString(downloadDir, text, nil)
		if err != nil {
			lb.error(fmt.Sprintf("Search download failed: %v", err))
			broadcastToClients(sess.getClients(), newErrorResponse("Error when downloading search results."))
			return
		}

		bookResults, parseErrors, err := core.ParseSearchFile(extractedPath)
		if err != nil {
			broadcastToClients(sess.getClients(), newErrorResponse("Error when parsing search results."))
			return
		}
		rawResults, _ := os.ReadFile(extractedPath)

		if len(bookResults) == 0 && len(parseErrors) == 0 {
			sess.noResultsHandler()(text)
			return
		}

		lb.info(fmt.Sprintf("🔍 Search results: %d found, %d unparseable", len(bookResults), len(parseErrors)))
		broadcastToClients(sess.getClients(), newSearchResponse(bookResults, parseErrors, string(rawResults)))
		os.Remove(extractedPath)

		// Record the search in server-side history.
		if sess.query != "" {
			srv.searchHistory.Add(sess.query)
		}
	}
}

// bookResultHandler implements the staging→post-process→prompt→confirm→move flow.
// It is called by the IRC reader goroutine (which runs for the session lifetime) and
// saves to the staged store when no client is connected or the client disconnects.
func (sess *session) bookResultHandler(
	config Config,
	lb *logBuffer,
	stagedStore *StagedBookStore,
	seriesReg *SeriesRegistry,
	srv *server,
) core.HandlerFunc {
	return func(text string) {
		dir := config.DownloadDir

		// Pop the slot handle for this download (FIFO — matches the order IRC requests
		// were sent). Both this handler and the per-job timeout goroutine hold a reference;
		// slotHandle.release() is idempotent via sync.Once.
		var handle *slotHandle
		select {
		case handle = <-sess.pendingSlots:
		default:
		}

		if err := staging.EnsureStagingDir(dir); err != nil {
			lb.error("Failed to create staging directory.")
			if handle != nil {
				handle.release()
			}
			return
		}
		stage := staging.StagingDir(dir)

		// DCC offer received — clear the "waiting for bot" UI state and signal transfer start.
		broadcastToClients(sess.getClients(), newDownloadWaitingClear())
		broadcastToClients(sess.getClients(), newDownloadStartedResponse())

		var ircFilenamePreview string
		if d, err := dcc.ParseString(text); err == nil {
			ircFilenamePreview = d.Filename
		}
		group := ircFilenamePreview
		if group == "" {
			group = fmt.Sprintf("dl-%d", time.Now().UnixMilli())
		}
		sess_lb := lb.session(group)
		sess_lb.info(fmt.Sprintf("⬇️  Downloading: %s", ircFilenamePreview))

		// 1. Download to staging.
		extractedPath, err := core.DownloadExtractDCCString(stage, text, nil)
		if err != nil {
			sess_lb.error(fmt.Sprintf("Download failed: %v", err))
			broadcastToClients(sess.getClients(), newErrorResponse("Error when downloading book."))
			if handle != nil {
				handle.release()
			}
			return
		}

		// File is safely on disk — release the download slot immediately so the
		// queue can start the next IRC request while this one is being renamed.
		if handle != nil {
			handle.release()
		}

		size := fileSizeMB(extractedPath)
		ircFilename := filepath.Base(extractedPath)
		sess_lb.infoDetail(
			fmt.Sprintf("📥 Downloaded: %s (%s)", ircFilename, size),
			fmt.Sprintf("File: %s\nSize: %s\nStaged at: %s", ircFilename, size, extractedPath),
		)

		var stagedOriginalPath string
		if config.DevMode {
			stagedOriginalPath = staging.OriginalCopyPath(extractedPath)
			if err := staging.CopyFile(extractedPath, stagedOriginalPath); err != nil {
				sess_lb.warn(fmt.Sprintf("Could not preserve original download: %v", err))
				stagedOriginalPath = ""
			}
		}

		// 2. Run post-processor.
		if len(config.PostProcessCmd) > 0 {
			broadcastToClients(sess.getClients(), newPostProcessStartedResponse())
		}
		runPostProcess(config.PostProcessCmd, extractedPath, sess_lb)

		// 3. Read EPUB metadata and cover.
		var meta *core.EPUBMetadata
		var coverBase64, coverMime string
		if strings.EqualFold(filepath.Ext(extractedPath), ".epub") {
			if m, err := core.ReadEPUBMetadata(extractedPath); err == nil {
				meta = m
			}
			if imgBytes, mime, err := core.ExtractCoverImage(extractedPath); err == nil && imgBytes != nil {
				coverBase64 = base64.StdEncoding.EncodeToString(imgBytes)
				coverMime = mime
			}
		}

		// 4. Build rename options.
		options := staging.BuildOptions(ircFilename, meta, config.ReplaceSpace)

		// saveToStaged saves the book to the staged store and cleans up.
		saveToStaged := func() {
			staged := &StagedBook{
				ID:           uuid.New().String(),
				StagedPath:   extractedPath,
				IRCFilename:  ircFilename,
				Metadata:     meta,
				Options:      options,
				ReplaceSpace: config.ReplaceSpace,
				CoverBase64:  coverBase64,
				CoverMime:    coverMime,
				StagedAt:     time.Now(),
			}
			if err := stagedStore.Add(staged); err != nil {
				os.Remove(extractedPath)
			}
			if stagedOriginalPath != "" {
				os.Remove(stagedOriginalPath)
			}
			srv.broadcastStagedCount()
		}

		// 5. Serialise the rename dialog.
		// Only one RENAME_PROMPT is shown at a time — if another download already has
		// the rename dialog open, block here until it finishes (or the session ends).
		select {
		case <-sess.renameMu:
			defer func() { sess.renameMu <- struct{}{} }()
		case <-sess.ctx.Done():
			saveToStaged()
			return
		}

		// Re-read any client after acquiring the mutex — if none connected, save to staged.
		currentClient := sess.getAnyClient()
		if currentClient == nil {
			saveToStaged()
			return
		}

		// Client is connected — send RENAME_PROMPT and wait.
		safeSend(currentClient, RenamePromptResponse{
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

		var choice RenameChoice
		select {
		case choice = <-currentClient.renameConfirm:
		case <-time.After(30 * time.Minute):
			sess_lb.warn(fmt.Sprintf("Rename timed out — keeping IRC filename: %s", ircFilename))
			choice = RenameChoice{OptionID: "keep"}
		case <-currentClient.ctx.Done():
			// Client disconnected mid-rename — save to staged store.
			saveToStaged()
			return
		}

		// Handle "queue for later" choice.
		if choice.OptionID == "queue_later" {
			saveToStaged()
			return
		}

		// 6. Move from staging to final path.
		optionLabel := choice.OptionID
		for _, opt := range options {
			if opt.ID == choice.OptionID {
				optionLabel = opt.Label
				break
			}
		}
		finalPath := staging.ResolveFinalPath(dir, choice, ircFilename, meta, config.ReplaceSpace)

		if err := staging.MoveFile(extractedPath, finalPath); err != nil {
			sess_lb.error(fmt.Sprintf("Failed to move file: %v", err))
			finalPath = extractedPath
		}
		if stagedOriginalPath != "" {
			originalFinalPath := staging.OriginalCopyPath(finalPath)
			if err := staging.MoveFile(stagedOriginalPath, originalFinalPath); err != nil {
				sess_lb.warn(fmt.Sprintf("Failed to save original copy: %v", err))
			} else {
				relOrig, _ := filepath.Rel(dir, originalFinalPath)
				sess_lb.infoDetail(
					fmt.Sprintf("🧪 Original preserved: %s", filepath.ToSlash(relOrig)),
					fmt.Sprintf("Path: %s", originalFinalPath),
				)
			}
		}

		// 7. Optionally rewrite EPUB internal metadata.
		if choice.RewriteMetadata && strings.EqualFold(filepath.Ext(finalPath), ".epub") {
			if err := staging.RewriteEPUBMetadata(finalPath, choice.Title, choice.Author, choice.Series, choice.SeriesIndex, choice.ClearSeries, choice.ClearSeriesIndex); err != nil {
				sess_lb.warn(fmt.Sprintf("Metadata rewrite failed: %v", err))
			} else {
				sess_lb.infoDetail("✏️  Metadata rewritten",
					fmt.Sprintf("Author: %s\nTitle: %s\nSeries: %s\nBook #: %s",
						choice.Author, choice.Title, choice.Series, choice.SeriesIndex))
			}
		}

		// 8. Track series name for autocomplete.
		if choice.Series != "" {
			seriesReg.AddIfNew(choice.Series)
		}

		// 9. Log and notify.
		rel, _ := filepath.Rel(dir, finalPath)
		relSlash := filepath.ToSlash(rel)
		savedDetail := fmt.Sprintf("Option: %s\nAuthor: %s\nTitle: %s\nSeries: %s\nBook #: %s\nPath: %s",
			optionLabel, choice.Author, choice.Title, choice.Series, choice.SeriesIndex, finalPath)
		sess_lb.infoDetail(fmt.Sprintf("✅ Saved [%s]: %s", optionLabel, relSlash), savedDetail)

		broadcastToClients(sess.getClients(), newDownloadResponse(finalPath, dir))
	}
}

func (sess *session) noResultsHandler() core.HandlerFunc {
	return func(_ string) {
		broadcastToClients(sess.getClients(), newErrorResponse("No results found for the query."))
	}
}

func (sess *session) badServerHandler() core.HandlerFunc {
	return func(_ string) {
		broadcastToClients(sess.getClients(), newErrorResponse("Server is not available. Try another one."))
	}
}

func (sess *session) searchAcceptedHandler() core.HandlerFunc {
	return func(_ string) {
		broadcastToClients(sess.getClients(), newStatusResponse(NOTIFY, "Search accepted into the queue."))
	}
}

func (sess *session) matchesFoundHandler() core.HandlerFunc {
	return func(num string) {
		broadcastToClients(sess.getClients(), newStatusResponse(NOTIFY, fmt.Sprintf("Found %s results for your query.", num)))
	}
}

func (sess *session) pingHandler() core.HandlerFunc {
	return func(serverUrl string) {
		sess.irc.Pong(serverUrl)
	}
}

func (sess *session) versionHandler(version string) core.HandlerFunc {
	return func(line string) {
		core.SendVersionInfo(sess.irc, line, version)
	}
}

func (sess *session) userListHandler(srv *server) core.HandlerFunc {
	return func(text string) {
		servers := core.ParseServers(text)
		sess.setServerList(servers)

		// Notify all connected clients of updated server list with timestamp.
		broadcastToClients(sess.getClients(), newServerListResponse(servers))
	}
}
