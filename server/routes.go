package server

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/evan-buss/openbooks/irc"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

//go:embed app/dist
var reactClient embed.FS

func (server *server) registerRoutes() *chi.Mux {
	router := chi.NewRouter()
	router.Handle("/*", server.staticFilesHandler("app/dist"))
	router.Get("/ws", server.serveWs())
	router.Get("/stats", server.statsHandler())
	router.Get("/servers", server.serverListHandler())
	router.Get("/version", server.versionHandler())

	router.Get("/logs", server.logsHandler())

	router.Group(func(r chi.Router) {
		r.Use(server.requireUser)
		r.Get("/library", server.getAllBooksHandler())
		r.Delete("/library/*", server.deleteBooksHandler())
		r.Get("/library/*", server.getBookHandler())
	})

	return router
}

// serveWs handles websocket requests from the peer.
func (server *server) serveWs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("OpenBooks")
		if errors.Is(err, http.ErrNoCookie) {
			cookie = &http.Cookie{
				Name:     "OpenBooks",
				Value:    uuid.New().String(),
				Secure:   false,
				HttpOnly: true,
				Expires:  time.Now().Add(time.Hour * 24 * 7),
				SameSite: http.SameSiteStrictMode,
			}
			w.Header().Add("Set-Cookie", cookie.String())
		}

		userId, err := uuid.Parse(cookie.Value)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		upgrader.CheckOrigin = func(req *http.Request) bool {
			return true
		}

		conn, err := upgrader.Upgrade(w, r, w.Header())
		if err != nil {
			server.log.Println(err)
			return
		}

		uniqueUsername := server.generateUniqueUsername()
		client := &Client{
			conn:          conn,
			send:          make(chan interface{}, 128),
			uuid:          userId,
			irc:           irc.New(uniqueUsername, server.config.UserAgent),
			log:           log.New(os.Stdout, fmt.Sprintf("CLIENT (%s): ", uniqueUsername), log.LstdFlags|log.Lmsgprefix),
			ctx:           context.Background(),
			downloadQueue: make(chan downloadJob, 50),
			downloadDone:  make(chan struct{}, 1),
			renameConfirm: make(chan RenameChoice, 1),
		}

		server.log.Printf("Client connected from %s\n", conn.RemoteAddr().String())
		client.log.Println("New client created.")

		server.register <- client

		go server.writePump(client)
		go server.readPump(client)
	}
}

func (server *server) generateUniqueUsername() string {
	baseUsername := server.config.UserName
	suffix := fmt.Sprintf("_%s", uuid.New().String()[:4])

	// Truncate base if needed (IRC has username limits)
	maxBaseLen := 12
	if len(baseUsername) > maxBaseLen {
		baseUsername = baseUsername[:maxBaseLen]
	}
	return baseUsername + suffix
}

func (server *server) staticFilesHandler(assetPath string) http.Handler {
	// update the embedded file system's tree so that index.html is at the root
	app, err := fs.Sub(reactClient, assetPath)
	if err != nil {
		server.log.Println(err)
	}

	// strip the predefined base path and serve the static file
	return http.StripPrefix(server.config.Basepath, http.FileServer(http.FS(app)))
}

func (server *server) statsHandler() http.HandlerFunc {
	type statsReponse struct {
		UUID string `json:"uuid"`
		IP   string `json:"ip"`
		Name string `json:"name"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		result := make([]statsReponse, 0, len(server.clients))

		for _, client := range server.clients {
			details := statsReponse{
				UUID: client.uuid.String(),
				Name: client.irc.Username,
				IP:   client.conn.RemoteAddr().String(),
			}

			result = append(result, details)
		}

		json.NewEncoder(w).Encode(result)
	}
}

func (server *server) serverListHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(server.repository.servers)
	}
}

func (server *server) versionHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(newVersionInfo(
			server.config.Version,
			server.config.CommitSHA,
			server.config.BuildDate,
		))
	}
}

func (server *server) getAllBooksHandler() http.HandlerFunc {
	type download struct {
		Name         string    `json:"name"`
		Path         string    `json:"path"`
		DownloadLink string    `json:"downloadLink"`
		Time         time.Time `json:"time"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		libraryDir := server.config.DownloadDir
		output := make([]download, 0)

		err := filepath.WalkDir(libraryDir, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			// Skip hidden directories (including .staging)
			if d.IsDir() {
				if strings.HasPrefix(d.Name(), ".") {
					return filepath.SkipDir
				}
				return nil
			}
			name := d.Name()
			if strings.HasPrefix(name, ".") || filepath.Ext(name) == ".temp" {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				server.log.Println(err)
				return nil
			}
			// relPath is relative to DownloadDir, e.g. "Author/Series/Title/file.epub"
			relPath, _ := filepath.Rel(libraryDir, p)
			output = append(output, download{
				Name:         name,
				Path:         filepath.ToSlash(relPath),
				DownloadLink: path.Join("library", filepath.ToSlash(relPath)),
				Time:         info.ModTime(),
			})
			return nil
		})
		if err != nil {
			server.log.Printf("Unable to list books. %s\n", err)
		}

		w.Header().Add("Content-Type", "application/json")
		json.NewEncoder(w).Encode(output)
	}
}

func (server *server) getBookHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		relPath, _ := url.PathUnescape(chi.URLParam(r, "*"))
		bookPath := filepath.Join(server.config.DownloadDir, filepath.FromSlash(relPath))

		http.ServeFile(w, r, bookPath)
	}
}

func (server *server) logsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		json.NewEncoder(w).Encode(server.logBuf.all())
	}
}

func (server *server) deleteBooksHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		relPath, err := url.PathUnescape(chi.URLParam(r, "*"))
		if err != nil {
			server.log.Printf("Error unescaping path: %s\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		bookPath := filepath.Join(server.config.DownloadDir, filepath.FromSlash(relPath))
		err = os.Remove(bookPath)
		if err != nil {
			server.log.Printf("Error deleting book file: %s\n", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
}
