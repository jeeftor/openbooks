package server

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/evan-buss/openbooks/mcp"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/rs/cors"
)

type server struct {
	// Shared app configuration
	config *Config

	// Shared data
	repository *Repository

	// Registered clients (active WebSocket connections).
	clients map[uuid.UUID]*Client

	// IRC sessions keyed by browser UUID — persist beyond WebSocket disconnects
	// so downloads continue in the background.
	sessions   map[uuid.UUID]*session
	sessionsMu sync.RWMutex

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client

	// broadcastCh sends a message to all currently connected clients.
	broadcastCh chan interface{}

	log *log.Logger

	// In-memory activity log (ring buffer)
	logBuf *logBuffer

	// Cached release update checker used by /version.
	updateChecker updateChecker

	// stagedBooks persists books that have been downloaded but not yet renamed.
	stagedBooks *StagedBookStore

	// seriesRegistry tracks known series names for autocomplete.
	seriesRegistry *SeriesRegistry

	// searchHistory records recent searches, persisted to disk.
	searchHistory *SearchHistoryStore
}

// Config contains settings for server
type Config struct {
	Log               bool
	Port              string
	UserName          string
	DownloadDir       string
	Basepath          string
	Server            string
	EnableTLS         bool
	SearchTimeout     time.Duration
	SearchBot         string
	UserAgent         string
	Version           string
	CommitSHA         string
	BuildDate         string
	OrganizeDownloads bool
	ReplaceSpace      string
	PostProcessCmd    []string // command + args; file path appended automatically
	DevMode           bool
	EnableMCP         bool     // mount MCP server at /mcp
	MCPFormats        []string // file format filter for MCP searches (default: epub)
}

func New(config Config) *server {
	return &server{
		repository:  NewRepository(),
		config:      &config,
		logBuf:      newLogBuffer(500),
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		broadcastCh: make(chan interface{}, 32),
		clients:     make(map[uuid.UUID]*Client),
		sessions:    make(map[uuid.UUID]*session),
		log:         log.New(os.Stdout, "SERVER: ", log.LstdFlags|log.Lmsgprefix),
		updateChecker: newGitHubUpdateChecker(log.New(
			os.Stdout,
			"UPDATE: ",
			log.LstdFlags|log.Lmsgprefix,
		)),
	}
}

// initStores initialises the staged book store and series registry.
// Called from Start() after the books directory is guaranteed to exist.
func (server *server) initStores() {
	store, err := newStagedBookStore(server.config.DownloadDir)
	if err != nil {
		server.log.Fatalf("staged store init: %v", err)
	}
	server.stagedBooks = store
	server.seriesRegistry = newSeriesRegistry(server.config.DownloadDir)

	hist, err := newSearchHistoryStore(server.config.DownloadDir)
	if err != nil {
		server.log.Fatalf("search history store init: %v", err)
	}
	server.searchHistory = hist
}

// broadcastStagedCount sends the current staged books count to all connected clients.
func (server *server) broadcastStagedCount() {
	select {
	case server.broadcastCh <- newStagedBooksNotifyResponse(server.stagedBooks.Count()):
	default:
	}
}

// getOrCreateSession returns the IRC session for the given user UUID, creating one if needed.
func (server *server) getOrCreateSession(userID uuid.UUID) *session {
	server.sessionsMu.Lock()
	defer server.sessionsMu.Unlock()
	if sess, ok := server.sessions[userID]; ok {
		return sess
	}
	username := server.generateUniqueUsernameUnsafe(userID)
	sess := newSession(username, server.config.UserAgent)
	server.sessions[userID] = sess
	return sess
}

// getSession returns the IRC session for the given user UUID, or nil.
func (server *server) getSession(userID uuid.UUID) *session {
	server.sessionsMu.RLock()
	defer server.sessionsMu.RUnlock()
	return server.sessions[userID]
}

// Start instantiates the web server and opens the browser
func Start(config Config) {
	createBooksDirectory(config)

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)

	corsConfig := cors.Options{
		AllowCredentials: true,
		// Allow localhost and private network IPs for development
		AllowOriginFunc: func(origin string) bool {
			// Allow any localhost or private IP for development
			return true
		},
		AllowedHeaders: []string{"*"},
		AllowedMethods: []string{"GET", "DELETE"},
	}
	router.Use(cors.New(corsConfig).Handler)

	srv := New(config)
	srv.initStores()
	routes := srv.registerRoutes()

	ctx, cancel := context.WithCancel(context.Background())
	go srv.startClientHub(ctx)
	srv.registerGracefulShutdown(cancel)
	router.Mount(config.Basepath, routes)

	if config.EnableMCP {
		mcpSession, err := mcp.Connect(ctx, mcp.Config{
			UserName:       config.UserName + "_mcp",
			UserAgent:      config.UserAgent,
			Server:         config.Server,
			EnableTLS:      config.EnableTLS,
			SearchBot:      config.SearchBot,
			DownloadDir:    config.DownloadDir,
			Formats:        config.MCPFormats,
			PostProcessCmd: config.PostProcessCmd,
			ReplaceSpace:   config.ReplaceSpace,
			Log:            slog.New(slog.NewTextHandler(os.Stderr, nil)),
			ActivityLog: func(level, msg string) {
				switch level {
				case "error":
					srv.logBuf.error(msg)
				default:
					srv.logBuf.info(msg)
				}
			},
		})
		if err != nil {
			srv.log.Printf("MCP IRC connect failed: %v — MCP endpoint disabled", err)
		} else {
			router.Mount("/mcp", mcp.NewMCPHandler(mcpSession))
			srv.log.Printf("MCP endpoint:           http://127.0.0.1:%s%smcp", config.Port, config.Basepath)
		}
	}

	srv.log.Printf("Version: %s (commit: %s, built: %s)\n", config.Version, config.CommitSHA, config.BuildDate)
	srv.log.Printf("Listening on port:      %v", config.Port)
	srv.log.Printf("Base path:              %s", config.Basepath)
	srv.log.Printf("Download directory:     %s", config.DownloadDir)
	srv.log.Printf("Organize downloads:     %v", config.OrganizeDownloads)
	srv.log.Printf("Dev mode:               %v", config.DevMode)
	srv.log.Printf("IRC server:             %s (TLS: %v)", config.Server, config.EnableTLS)
	srv.log.Printf("Username:               %s", config.UserName)
	srv.log.Printf("Search bot:             %s", config.SearchBot)
	if len(config.PostProcessCmd) > 0 {
		srv.log.Printf("Post-process command:   %v", config.PostProcessCmd)
		validatePostProcessCmd(config.PostProcessCmd, srv.log)
	}
	srv.log.Fatal(http.ListenAndServe(":"+config.Port, router))
}

// The client hub is to be run in a goroutine and handles management of
// websocket client registrations.
func (server *server) startClientHub(ctx context.Context) {
	for {
		select {
		case client := <-server.register:
			// Handle reconnect: if the same UUID is already registered (e.g. after
			// a network drop), close the old connection so its goroutines exit.
			if old, exists := server.clients[client.uuid]; exists {
				old.conn.Close() // causes readPump ReadJSON to fail → defer fires
				close(old.send)  // causes writePump to exit cleanly
				server.log.Printf("Replaced stale connection for %s\n", old.username)
			}
			server.clients[client.uuid] = client
		case client := <-server.unregister:
			// Use pointer equality: if a new client replaced this one, skip the delete
			// so we don't evict the replacement from the map.
			if existing, ok := server.clients[client.uuid]; ok && existing == client {
				close(client.send)
				delete(server.clients, client.uuid)
			}
		case msg := <-server.broadcastCh:
			for _, client := range server.clients {
				safeSend(client, msg)
			}
		case <-ctx.Done():
			for _, client := range server.clients {
				close(client.send)
				delete(server.clients, client.uuid)
			}
			return
		}
	}
}

func (server *server) registerGracefulShutdown(cancel context.CancelFunc) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		server.log.Println("Graceful shutdown.")
		// Close the shutdown channel. Triggering all reader/writer WS handlers to close.
		cancel()
		time.Sleep(time.Second)
		os.Exit(0)
	}()
}

func createBooksDirectory(config Config) {
	err := os.MkdirAll(config.DownloadDir, os.FileMode(0755))
	if err != nil {
		panic(err)
	}
}
