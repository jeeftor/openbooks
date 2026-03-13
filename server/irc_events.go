package server

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/evan-buss/openbooks/core"
)

// organizeByMetadata moves an EPUB to a metadata-derived subdirectory under downloadDir/books/.
// Returns the final path (original path if unchanged or on error).
func (c *Client) organizeByMetadata(extractedPath string, config Config) string {
	if !config.OrganizeDownloads {
		return extractedPath
	}
	if filepath.Ext(extractedPath) != ".epub" {
		return extractedPath
	}

	meta, err := core.ReadEPUBMetadata(extractedPath)
	if err != nil || meta == nil || meta.Author == "" || meta.Title == "" {
		c.log.Printf("organizeByMetadata: skipping metadata organization for %s (err=%v)", filepath.Base(extractedPath), err)
		return extractedPath
	}

	rs := config.ReplaceSpace
	author := sanitizePathComponent(meta.Author, rs)
	title := sanitizePathComponent(meta.Title, rs)

	var targetDir string
	if meta.Series != "" {
		series := sanitizePathComponent(meta.Series, rs)
		targetDir = filepath.Join(config.DownloadDir, author, series, title)
	} else {
		targetDir = filepath.Join(config.DownloadDir, author, title)
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		c.log.Printf("organizeByMetadata: failed to create dir %s: %v", targetDir, err)
		return extractedPath
	}

	newPath := filepath.Join(targetDir, filepath.Base(extractedPath))
	if err := os.Rename(extractedPath, newPath); err != nil {
		c.log.Printf("organizeByMetadata: failed to move file: %v", err)
		return extractedPath
	}

	return newPath
}

func (server *server) NewIrcEventHandler(client *Client) core.EventHandler {
	handler := core.EventHandler{}
	handler[core.SearchResult] = client.searchResultHandler(server.config.DownloadDir)
	handler[core.BookResult] = client.bookResultHandler(*server.config)
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
func (c *Client) searchResultHandler(downloadDir string) core.HandlerFunc {
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
		c.send <- newSearchResponse(bookResults, parseErrors)

		err = os.Remove(extractedPath)
		if err != nil {
			c.log.Printf("Error deleting search results file: %v", err)
		}
	}
}

// bookResultHandler downloads the book file and sends it over the websocket
func (c *Client) bookResultHandler(config Config) core.HandlerFunc {
	return func(text string) {
		dir := config.DownloadDir
		extractedPath, err := core.DownloadExtractDCCString(dir, text, nil)
		if err != nil {
			c.log.Println(err)
			c.send <- newErrorResponse("Error when downloading book.")
			return
		}

		finalPath := c.organizeByMetadata(extractedPath, config)
		c.log.Printf("Downloaded book to: %s\n", finalPath)
		c.log.Printf("Sending book entitled '%s'.\n", filepath.Base(finalPath))
		c.send <- newDownloadResponse(finalPath, config.DownloadDir, config.DisableBrowserDownloads)
	}
}

// NoResults is called when the server returns that nothing was found for the query
func (c *Client) noResultsHandler(_ string) {
	c.send <- newErrorResponse("No results found for the query.")
}

// BadServer is called when the requested download fails because the server is not available
func (c *Client) badServerHandler(_ string) {
	c.send <- newErrorResponse("Server is not available. Try another one.")
}

// SearchAccepted is called when the user's query is accepted into the search queue
func (c *Client) searchAcceptedHandler(_ string) {
	c.send <- newStatusResponse(NOTIFY, "Search accepted into the queue.")
}

// MatchesFound is called when the server finds matches for the user's query
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
