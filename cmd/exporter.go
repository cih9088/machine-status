package cmd

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strconv"
	"time"

	fqdn "github.com/Showmax/go-fqdn"
	"github.com/spf13/cobra"
)

type ExporterOptions struct {
	Port int
}

type Cache struct {
	Data []byte
	Time time.Time
}

var (
	exporterOptions ExporterOptions

	cache = Cache{
		Time: time.Now(),
	}

	exporterCmd = &cobra.Command{
		Use:    "exporter",
		Short:  "machine-status exporter",
		Long:   `machine-status exporter`,
		Args:   cobra.NoArgs,
		Run:    exporterRun,
		Hidden: false,
	}
)

func exporterWSHandler(response http.ResponseWriter, request *http.Request) {
	log.Infof("Connected from %s", request.RemoteAddr)

	// Upgrade initial GET request to a websocket
	ws, err := upgrader.Upgrade(response, request, nil)
	if err != nil {
		log.Error(err)
	}
	// Make sure we close the connection when the function returns
	defer ws.Close()

	for {
		mt, message, err := ws.ReadMessage()
		if err != nil {
			log.Warn("Read from server is failed: ", err)
			break
		}
		log.Debugf("Received message from server: %s\n", message)

		out, err := ansi2html(cache.Data)
		err = ws.WriteMessage(mt, out)
		if err != nil {
			log.Warn("Write to server is failed: ", err)
			break
		}
	}
}

func homeConnections(response http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/" {
		http.Error(response, "404 not found.", http.StatusNotFound)
		return
	}
	switch request.Method {
	case "POST":
		// Call ParseForm() to parse the raw query and update request.PostForm and request.Form.
		if err := request.ParseForm(); err != nil {
			fmt.Fprintf(response, "ParseForm() err: %v", err)
			return
		}
		log.Infof("Post reqeust: request.PostFrom = %v\n", request.PostForm)
		response.Write(cache.Data)
	default:
		log.Warnf("%s is not suppored", request.Method)
	}
}

func ansi2html(data []byte) ([]byte, error) {
	cmd := exec.Command("bash", "-c", "./scripts/ansi2html --body-only")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Error(err)
	}
	defer stdin.Close()

	go func() {
		defer stdin.Close()
		io.WriteString(stdin, string(data))
	}()

	out, err := cmd.Output()
	return out, err
}

func init() {
	rootCmd.AddCommand(exporterCmd)
	exporterCmd.Flags().IntVar(&exporterOptions.Port, "port", 9200,
		"port to serve")
}

func exporterRun(cmd *cobra.Command, args []string) {
	go func(cache *Cache) {
		for {
			cmd := exec.Command("./scripts/sys-usage")
			var out bytes.Buffer
			cmd.Stdout = &out
			err := cmd.Run()
			if err != nil {
				log.Fatal(err)
			}
			cache.Time = time.Now()
			cache.Data = out.Bytes()
			log.Debugf("Cache update (%s)", cache.Time.String())
		}
	}(&cache)

	http.HandleFunc("/", homeConnections)
	http.HandleFunc("/ws", exporterWSHandler)

	log.Infof("Serving server on %s with port %d\n", fqdn.Get(), exporterOptions.Port)
	err := http.ListenAndServe(":"+strconv.Itoa(exporterOptions.Port), nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
