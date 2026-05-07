package server

import (
	"encoding/json"
	"fmt"
	"math"
	"path"
	"strings"

	"github.com/evan-buss/openbooks/core"
)

//go:generate stringer -type=MessageType
type MessageType int

// Available commands. These are sent via integers starting at 1
const (
	STATUS MessageType = iota
	CONNECT
	SEARCH
	DOWNLOAD
	RATELIMIT
	RENAME_PROMPT        // server → client: book staged, awaiting rename decision
	RENAME_CONFIRM       // client → server: user's rename decision
	DOWNLOAD_WAITING     // server → client: IRC request sent, waiting for DCC response
	DOWNLOAD_STARTED     // server → client: DCC accepted, file transfer in progress
	POST_PROCESS_STARTED // server → client: post-processing (ebook-polish) running
)

type NotificationType int

const (
	NOTIFY NotificationType = iota
	SUCCESS
	WARNING
	DANGER
)

type StatusResponse struct {
	MessageType      MessageType      `json:"type"`
	NotificationType NotificationType `json:"appearance"`
	Title            string           `json:"title"`
	Detail           string           `json:"detail"`
}

// Request in a generic structure for all requests from the websocket client
type Request struct {
	MessageType MessageType     `json:"type"`
	Payload     json.RawMessage `json:"payload"`
}

// ConnectionRequest is a request to start the IRC server
type ConnectionRequest struct{}

// SearchRequest is a request that sends a search request to the IRC server for a specific query
type SearchRequest struct {
	Query string `json:"query"`
}

// DownloadRequest is a request to download a specific book from the IRC server
type DownloadRequest struct {
	Book   string `json:"book"`
	Author string `json:"author,omitempty"`
	Title  string `json:"title,omitempty"`
}

// ConnectionResponse
type ConnectionResponse struct {
	StatusResponse
	Name string `json:"name"`
}

// SearchResponse is a response that is sent containing BookDetails objects that matched the query
type SearchResponse struct {
	StatusResponse
	Books  []core.BookDetail `json:"books"`
	Errors []core.ParseError `json:"errors"`
	Raw    string            `json:"raw,omitempty"`
}

// DownloadResponse is a response that sends the requested book to the client
type DownloadResponse struct {
	StatusResponse
	Name         string `json:"name"`
	DownloadPath string `json:"downloadPath"`
}

// RenameOption is one naming choice shown in the rename modal.
type RenameOption struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Preview     string `json:"preview"`     // path relative to downloadDir, forward slashes
	IsOrganized bool   `json:"isOrganized"` // true if it creates subdirectories
}

// RenamePromptResponse is sent when a book is staged and ready for the user to name.
type RenamePromptResponse struct {
	StatusResponse
	IRCFilename  string             `json:"ircFilename"`
	Metadata     *core.EPUBMetadata `json:"metadata,omitempty"`
	Options      []RenameOption     `json:"options"`
	ReplaceSpace string             `json:"replaceSpace"`
	CoverBase64  string             `json:"coverBase64,omitempty"` // base64-encoded cover image
	CoverMime    string             `json:"coverMime,omitempty"`   // e.g. "image/jpeg"
}

// RenameConfirmRequest is sent by the client with the user's rename decision.
type RenameConfirmRequest struct {
	OptionID        string `json:"optionId"`
	CustomName      string `json:"customName"`
	FileName        string `json:"fileName,omitempty"`
	RewriteMetadata bool   `json:"rewriteMetadata"`
	// Metadata fields to write (may differ from extracted if user edited them)
	Author      string `json:"author,omitempty"`
	Title       string `json:"title,omitempty"`
	Series      string `json:"series,omitempty"`
	SeriesIndex string `json:"seriesIndex,omitempty"`
}

// RenameChoice is the internal representation passed from the WS handler to bookResultHandler.
type RenameChoice struct {
	OptionID        string
	CustomName      string
	FileName        string
	RewriteMetadata bool
	Author          string
	Title           string
	Series          string
	SeriesIndex     string
}

func newRateLimitResponse(remainingSeconds float64) StatusResponse {
	wait := math.Round(remainingSeconds)
	units := "seconds"
	if wait == 1 {
		units = "second"
	}

	return StatusResponse{
		MessageType:      RATELIMIT,
		NotificationType: WARNING,
		Title:            "You are searching too frequently!",
		Detail:           fmt.Sprintf("Please wait %v %s to submit another search.", wait, units),
	}
}

func newSearchResponse(results []core.BookDetail, errors []core.ParseError, raw string) SearchResponse {
	detail := fmt.Sprintf("There were %v parsing errors.", len(errors))
	if len(errors) == 1 {
		detail = "There was 1 parsing error."
	}
	return SearchResponse{
		StatusResponse: StatusResponse{
			MessageType:      SEARCH,
			NotificationType: SUCCESS,
			Title:            fmt.Sprintf("%v Search Results Received", len(results)),
			Detail:           detail,
		},
		Books:  results,
		Errors: errors,
		Raw:    raw,
	}
}

func newDownloadResponse(filePath string, downloadDir string) DownloadResponse {
	// Show path relative to the download root so the user knows where the file landed.
	relPath := strings.TrimPrefix(filePath, downloadDir+"/")

	return DownloadResponse{
		StatusResponse: StatusResponse{
			MessageType:      DOWNLOAD,
			NotificationType: SUCCESS,
			Title:            fmt.Sprintf("Book received: %s", path.Base(filePath)),
			Detail:           relPath,
		},
	}
}

func newStatusResponse(notificationType NotificationType, title string) StatusResponse {
	return StatusResponse{
		MessageType:      STATUS,
		NotificationType: notificationType,
		Title:            title,
	}
}

// DownloadWaitingResponse is sent when an IRC download request has been dispatched
// and we are waiting for the bot to send the DCC offer. Active=false clears the UI state.
type DownloadWaitingResponse struct {
	StatusResponse
	Active      bool   `json:"active"`
	Bot         string `json:"bot,omitempty"`
	BookTitle   string `json:"bookTitle,omitempty"`
	TimeoutSecs int    `json:"timeoutSecs,omitempty"`
}

func newDownloadWaitingResponse(bot, title string) DownloadWaitingResponse {
	return DownloadWaitingResponse{
		StatusResponse: StatusResponse{MessageType: DOWNLOAD_WAITING},
		Active:         true,
		Bot:            bot,
		BookTitle:      title,
		TimeoutSecs:    300,
	}
}

func newDownloadWaitingClear() DownloadWaitingResponse {
	return DownloadWaitingResponse{
		StatusResponse: StatusResponse{MessageType: DOWNLOAD_WAITING},
		Active:         false,
	}
}

func newDownloadStartedResponse() StatusResponse {
	return StatusResponse{MessageType: DOWNLOAD_STARTED}
}

func newPostProcessStartedResponse() StatusResponse {
	return StatusResponse{MessageType: POST_PROCESS_STARTED}
}

func newErrorResponse(title string) StatusResponse {
	return StatusResponse{
		MessageType:      STATUS,
		NotificationType: DANGER,
		Title:            title,
	}
}
