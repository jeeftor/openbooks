package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/jeeftor/openbooks/desktop"
	"github.com/jeeftor/openbooks/server"
	"github.com/spf13/cobra"
)

// version will always match the GitHub release versions.
var version = "dev"

// commitSHA is the git commit hash, injected at build time
var commitSHA = "unknown"

// buildDate is the build timestamp, injected at build time
var buildDate = "unknown"

// We only increment ircVersion when irc admins require a fix to be made.
// They can block / permit certain version numbers. ircVersion is the current permitted
// version number.
var ircVersion = "4.3.0"

type GlobalFlags struct {
	UserName              string
	Server                string
	Log                   bool
	SearchBot             string
	SearchBotFallback     string
	SearchResponseTimeout time.Duration
	SearchResultTimeout   time.Duration
	EnableTLS             bool
	UserAgent             string
}

var debug bool
var globalFlags GlobalFlags
var desktopConfig server.Config

func init() {
	desktopCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug mode.")
	desktopCmd.PersistentFlags().StringVarP(&globalFlags.UserName, "name", "n", "", "Username used to connect to IRC server.")
	desktopCmd.PersistentFlags().StringVarP(&globalFlags.Server, "server", "s", "irc.irchighway.net:6697", "IRC server to connect to.")
	desktopCmd.PersistentFlags().BoolVar(&globalFlags.EnableTLS, "tls", true, "Connect to server using TLS.")
	desktopCmd.PersistentFlags().BoolVarP(&globalFlags.Log, "log", "l", false, "Save raw IRC logs for each client connection.")
	desktopCmd.PersistentFlags().StringVar(&globalFlags.SearchBot, "searchbot", "search", "The IRC bot that handles search queries. Try 'searchook' if 'search' is down.")
	desktopCmd.PersistentFlags().StringVar(&globalFlags.SearchBotFallback, "searchbot-fallback", "searchook", "Fallback IRC bot — each search retries once on it when the primary times out, and it handles searches while the primary is marked down. Set to empty to disable.")
	desktopCmd.PersistentFlags().DurationVar(&globalFlags.SearchResponseTimeout, "search-response-timeout", 2*time.Minute, "Max wait for any response from the search bot before the search is declared timed out.")
	desktopCmd.PersistentFlags().DurationVar(&globalFlags.SearchResultTimeout, "search-result-timeout", 5*time.Minute, "Max wait for the results file after the search bot acknowledges a query.")
	desktopCmd.PersistentFlags().StringVarP(&globalFlags.UserAgent, "useragent", "u", fmt.Sprintf("OpenBooks %s", ircVersion), "UserAgent / Version Reported to IRC Server.")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Errorf("unable to determine $HOME directory %w", err))
	}
	downloadDir := filepath.Join(homeDir, "Downloads")

	desktopCmd.Flags().StringVarP(&desktopConfig.Port, "port", "p", "5228", "Set the local network port for browser mode.")
	desktopCmd.Flags().IntP("rate-limit", "r", 30, "The number of seconds to wait between searches to reduce strain on IRC search servers. Minimum is 10 seconds.")
	desktopCmd.Flags().StringVarP(&desktopConfig.DownloadDir, "dir", "d", downloadDir, "The directory where eBooks are saved.")
}

var desktopCmd = &cobra.Command{
	Use:   "openbooks-abs",
	Short: "Quickly and easily download eBooks from IRCHighway.",
	Long:  "Runs openbooks-abs in desktop mode. This allows you to run openbooks-abs like a regular desktop application. This functionality utilizes your OS's native browser renderer and as such may not work on certain operating systems.",
	PreRun: func(cmd *cobra.Command, args []string) {
		bindGlobalServerFlags(&desktopConfig)
		rateLimit, _ := cmd.Flags().GetInt("rate-limit")
		ensureValidRate(rateLimit, &desktopConfig)
		desktopConfig.Basepath = "/"
	},
	Run: func(cmd *cobra.Command, args []string) {
		if debug {
			spew.Dump(desktopConfig)
		}

		go server.Start(desktopConfig)
		desktop.StartWebView(fmt.Sprintf("http://127.0.0.1:%s", path.Join(desktopConfig.Port+desktopConfig.Basepath)), debug)
	},
	Version: version,
}

func main() {
	// Don't block if launched from explorer.
	cobra.MousetrapHelpText = ""

	if err := desktopCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
