package cmd

import (
	"fmt"
	"net/http"
	"net/url"
	"path"
	"runtime"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type RootOptions struct {
	Debug bool
}

var (
	rootOptions = RootOptions{}

	log *logrus.Logger

	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin, ok := r.Header["Origin"]
			if !ok {
				return true
			}
			u, _ := url.Parse(origin[0])
			host := r.Host
			if strings.HasPrefix(u.Host, host) {
				return true
			}
			return false
		},
	}

	rootCmd = &cobra.Command{
		Use:   "machine-status",
		Short: "machine-status is a simple dashboard for multiple machines with nvidia GPU",
		Long:  `machine-status is a simple dashboard for multiple machines with nvidia GPU`,
	}
)

// Execute executes the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	log = logrus.New()
	log.SetReportCaller(true)
	log.Formatter = &logrus.TextFormatter{
		FullTimestamp: true,
		CallerPrettyfier: func(f *runtime.Frame) (string, string) {
			filename := path.Base(f.File)
			return fmt.Sprintf("%s()", path.Base(f.Function)),
				fmt.Sprintf(" %s:%d", filename, f.Line)
		},
	}

	rootCmd.PersistentFlags().BoolVar(
		&rootOptions.Debug, "debug",
		false, "Debug mode")
}

func check(e error) {
	if e != nil {
		log.Fatal(e)
	}
}
