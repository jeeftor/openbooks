package server

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

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

	// Registered clients.
	clients map[uuid.UUID]*Client

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client

	log *log.Logger

	// In-memory activity log (ring buffer)
	logBuf *logBuffer

	// Mutex to guard the lastSearch timestamp
	lastSearchMutex sync.Mutex

	// The time the last search was performed. Used to rate limit searches.
	lastSearch time.Time
}

// Config contains settings for server
type Config struct {
	Log                     bool
	Port                    string
	UserName                string
	Persist                 bool
	DownloadDir             string
	Basepath                string
	Server                  string
	EnableTLS               bool
	SearchTimeout           time.Duration
	SearchBot               string
	DisableBrowserDownloads bool
	UserAgent               string
	Version                 string
	OrganizeDownloads       bool
	ReplaceSpace            string
}

func New(config Config) *server {
	return &server{
		repository: NewRepository(),
		config:     &config,
		logBuf:     newLogBuffer(500),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[uuid.UUID]*Client),
		log:        log.New(os.Stdout, "SERVER: ", log.LstdFlags|log.Lmsgprefix),
	}
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

	server := New(config)
	routes := server.registerRoutes()

	ctx, cancel := context.WithCancel(context.Background())
	go server.startClientHub(ctx)
	server.registerGracefulShutdown(cancel)
	router.Mount(config.Basepath, routes)

	server.log.Printf("Base Path: %s\n", config.Basepath)
	server.log.Printf("OpenBooks is listening on port %v", config.Port)
	server.log.Printf("Download Directory: %s\n", config.DownloadDir)
	server.log.Printf("Open http://localhost:%v%s in your browser.", config.Port, config.Basepath)
	server.log.Printf("Persist downloads:      %v", config.Persist)
	server.log.Printf("Organize downloads:     %v", config.OrganizeDownloads)
	server.log.Printf("IRC server:             %s (TLS: %v)", config.Server, config.EnableTLS)
	server.log.Printf("Username:               %s", config.UserName)
	server.log.Printf("Search bot:             %s", config.SearchBot)
	server.log.Printf("Browser downloads:      %v", !config.DisableBrowserDownloads)
	server.log.Fatal(http.ListenAndServe(":"+config.Port, router))
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
				server.log.Printf("Replaced stale connection for %s\n", old.irc.Username)
			}
			server.clients[client.uuid] = client
		case client := <-server.unregister:
			// Use pointer equality: if a new client replaced this one, skip the delete
			// so we don't evict the replacement from the map.
			if existing, ok := server.clients[client.uuid]; ok && existing == client {
				_, cancel := context.WithCancel(client.ctx)
				close(client.send)
				cancel()
				delete(server.clients, client.uuid)
			}
		case <-ctx.Done():
			for _, client := range server.clients {
				_, cancel := context.WithCancel(client.ctx)
				close(client.send)
				cancel()
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
