package main

import (
	"os"
	"path"
	"path/filepath"

	"github.com/evan-buss/openbooks/server"
	"github.com/evan-buss/openbooks/util"

	"github.com/spf13/cobra"
)

var openBrowser = false
var serverConfig server.Config
var replaceSpace string

func init() {
	desktopCmd.AddCommand(serverCmd)

	serverCmd.Flags().StringVarP(&serverConfig.Port, "port", "p", "5228", "Set the local network port for browser mode.")
	serverCmd.Flags().IntP("rate-limit", "r", 10, "The number of seconds to wait between searches to reduce strain on IRC search servers. Minimum is 10 seconds.")
	serverCmd.Flags().StringVar(&serverConfig.Basepath, "basepath", "/", `Base path where the application is accessible. For example "/openbooks/".`)
	serverCmd.Flags().BoolVarP(&openBrowser, "browser", "b", false, "Open the browser on server start.")
	serverCmd.Flags().StringVarP(&serverConfig.DownloadDir, "dir", "d", filepath.Join(os.TempDir(), "openbooks"), "The directory where eBooks are saved.")
	serverCmd.Flags().BoolVar(&serverConfig.OrganizeDownloads, "organize-downloads", false, "Organize downloads into author/title/FILE subdirectories.")
	serverCmd.Flags().BoolVar(&serverConfig.DevMode, "dev", false, "Keep a raw .orig copy beside each saved download for local testing.")
	serverCmd.Flags().StringVar(&replaceSpace, "replace-space", "", "Replace spaces in author/title directory names with this character (e.g. '.', '-', '_').")
	serverCmd.Flags().StringSliceVar(&serverConfig.PostProcessCmd, "post-process-cmd", nil, "Command to run after each book download. File path is appended as last argument. Example: --post-process-cmd 'calibre-polish,--embed-fonts,--smarten-punctuation'")
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Run OpenBooks in server mode.",
	Long:  "Run OpenBooks in server mode. This allows you to use a web interface to search and download eBooks.",
	PreRun: func(cmd *cobra.Command, args []string) {
		bindGlobalServerFlags(&serverConfig)
		rateLimit, _ := cmd.Flags().GetInt("rate-limit")
		ensureValidRate(rateLimit, &serverConfig)
		// If cli flag isn't set (default value) check for the presence of an
		// environment variable and use it if found.
		if serverConfig.Basepath == cmd.Flag("basepath").DefValue {
			if envPath, present := os.LookupEnv("BASE_PATH"); present {
				serverConfig.Basepath = envPath
			}
		}
		serverConfig.Basepath = sanitizePath(serverConfig.Basepath)
	},
	Run: func(cmd *cobra.Command, args []string) {
		serverConfig.ReplaceSpace = replaceSpace
		serverConfig.Version = version
		serverConfig.CommitSHA = commitSHA
		serverConfig.BuildDate = buildDate
		if openBrowser {
			browserUrl := "http://127.0.0.1:" + path.Join(serverConfig.Port+serverConfig.Basepath)
			util.OpenBrowser(browserUrl)
		}

		server.Start(serverConfig)
	},
}
