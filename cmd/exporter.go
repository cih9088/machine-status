package cmd

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	fqdn "github.com/Showmax/go-fqdn"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type Cache struct {
	Data []byte
	Time time.Time
}

var (
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
	case "GET":
		// Call ParseForm() to parse the raw query and update request.PostForm and request.Form.
		if err := request.ParseForm(); err != nil {
			fmt.Fprintf(response, "ParseForm() err: %v", err)
			return
		}
		log.Infof("Get reqeust: \n")
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
	viper.SetEnvPrefix("mstat")
	viper.AutomaticEnv()

	rootCmd.AddCommand(exporterCmd)
	exporterCmd.Flags().Int("port", 9200, "port to serve")
	exporterCmd.Flags().String("mapping", "", "mapping between username and UID")
	exporterCmd.Flags().BoolP("show-user", "u", false, "show user name")
	exporterCmd.Flags().BoolP("show-pid", "p", false, "show PID")
	exporterCmd.Flags().BoolP("show-power", "w", false, "show power usage")
	exporterCmd.Flags().BoolP("show-cmd", "c", false, "show command of the process")
	exporterCmd.Flags().BoolP("show-fan", "f", false, "show fan speed")
	viper.BindPFlags(exporterCmd.Flags())

	replacer := strings.NewReplacer("-", "_")
	viper.SetEnvKeyReplacer(replacer)
}

func exporterRun(cmd *cobra.Command, args []string) {

	gpustatArgs := ""
	for _, key := range viper.AllKeys() {
		if key == "port" || key == "mapping" {
			continue
		}
		if viper.GetBool(key) {
			gpustatArgs += "--" + key + " "
		}
	}

	if viper.GetString("mapping") != "" {
		mappings := strings.Split(strings.TrimSpace(viper.GetString("mapping")), " ")

		f, err := os.Create("./mapping.txt")
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()

		for i := 0; i < len(mappings); i++ {
			mapping := strings.Split(mappings[i], ":")

			fmt.Println(strings.Join(mapping, ":"))
			f.WriteString(strings.Join(mapping, ":") + "\n")
		}
	}

	if rootOptions.Debug {
		log.SetLevel(logrus.DebugLevel)
	}

	go func(cache *Cache) {
		for {
			cmd := exec.Command("./scripts/sys-usage", gpustatArgs)
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

	log.Infof("Serving server on %s with port %d\n", fqdn.Get(), viper.GetInt("port"))
	err := http.ListenAndServe(":"+viper.GetString("port"), nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
