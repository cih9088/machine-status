package cmd

import (
	"html/template"
	"net"
	"net/http"
	"os"
	"path"
	"strings"

	"golang.org/x/crypto/acme/autocert"

	fqdn "github.com/Showmax/go-fqdn"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type InsecureOptions struct {
	ServerOptions
}

var (
	insecureServerOptions InsecureOptions

	insecureServerCmd = &cobra.Command{
		Use:    "server",
		Short:  "machine-status web service",
		Long:   `machine-status web service`,
		Args:   cobra.NoArgs,
		Run:    insecureServerOptions.Run,
		Hidden: false,
	}
)

func (o *InsecureOptions) dashboardHandler(response http.ResponseWriter, request *http.Request) {
	log.Infof("Connected client from %s", request.RemoteAddr)

	target := ""
	if o.Wss {
		target += "wss://"
	} else {
		target += "ws://"
	}
	index := strings.Index(o.FQDN, "/")
	if index == -1 {
		target += o.FQDN + o.Rootpage + "/ws"
	} else {
		log.Panic(o.FQDN)
	}

	log.Infof("ws target: %s", target)

	machines := []IndexPageData{}

	for idx := range o.Machines {
		machine := o.Machines[idx]
		alias := o.Aliases[idx]
		isCollapse := "checked"
		if stringInSlice(machine, o.Collapses) {
			isCollapse = ""
		}

		machines = append(machines, IndexPageData{
			Machine:    machine,
			Alias:      alias,
			IsCollapse: isCollapse,
		})
	}

	page, err := template.ParseFiles("web/template/dashboard_simple.html")
	check(err)

	page.Execute(response, struct {
		Ws       string
		Web      string
		Interval int
		Machines []IndexPageData
	}{
		Ws:       target,
		Web:      o.Rootpage + "/web",
		Interval: o.Interval,
		Machines: machines,
	})
}

func init() {
	rootCmd.AddCommand(insecureServerCmd)
	insecureServerCmd.Flags().BoolVar(&insecureServerOptions.Wss, "wss", false,
		"whether use wss for websocket or not")
	insecureServerCmd.Flags().StringVar(&insecureServerOptions.HttpsKey, "https-key", "",
		"location of key to serve https")
	insecureServerCmd.Flags().StringVar(&insecureServerOptions.HttpsCrt, "https-crt", "",
		"location of crt to serve https")
	insecureServerCmd.Flags().BoolVar(&insecureServerOptions.LetsEntrypt, "letsencrypt", false,
		"whether use letsencrypt for https")
	insecureServerCmd.Flags().StringVar(&insecureServerOptions.FQDN, "fqdn", fqdn.Get(),
		"fully qualified domain name or ip address including port. If port is not specified, it assumes '80'. This should be accessable from clinets.")
	insecureServerCmd.Flags().StringVar(&insecureServerOptions.Rootpage, "root", "/",
		"root page for the http server")
	insecureServerCmd.Flags().IntVar(&insecureServerOptions.Interval, "interval", 1000,
		"refresh interval in milliseconds")
	insecureServerCmd.Flags().StringSliceVar(&insecureServerOptions.Machines, "machine", []string{},
		"comma seperated exporter machines with port (ex: 'host:9200' or 'host:9200->alias' with alias) ")
	insecureServerCmd.Flags().StringSliceVar(&insecureServerOptions.Collapses, "collapse", []string{},
		"comma seperated exporter machines with port (ex: 'host:9200') to collapse default")
}

// server main method
func (o *InsecureOptions) Run(cmd *cobra.Command, args []string) {
	// assert options
	if o.HttpsKey != "" && o.HttpsCrt != "" {
		key := path.Join("/tmp/certs", o.HttpsKey)
		crt := path.Join("/tmp/certs", o.HttpsCrt)
		if _, err := os.Stat(key); os.IsNotExist(err) {
			log.Panicf("Https key %s not found", key)
		}
		if _, err := os.Stat(crt); os.IsNotExist(err) {
			log.Panicf("Https crt %s not found", crt)
		}
		if o.LetsEntrypt {
			log.Warn("https-key and https-crt has higher priority than letsencrypt.")
		}
	} else if o.HttpsKey == "" && o.HttpsCrt == "" {
	} else {
		log.Panic("https-key and https-crt should be given")
	}

	// parse machine
	for idx, machine := range o.Machines {
		parsed := strings.Split(machine, "->")
		alias := ""
		if len(parsed) == 1 {
			machine = parsed[0]
			alias = parsed[0]
		} else {
			machine = parsed[0]
			alias = parsed[1]
		}
		o.Machines[idx] = machine
		o.Aliases = append(o.Aliases, alias)
		log.Infof("Machine mapping: %s -> %s", machine, alias)
	}

	if rootOptions.Debug {
		log.SetLevel(logrus.DebugLevel)
	}

	o.Rootpage = strings.Trim(o.Rootpage, "/")
	if len(o.Rootpage) != 0 {
		o.Rootpage = "/" + o.Rootpage
	}

	o.init()
	o.connectAll()
	go o.connectLoop()
	go o.fetchLoop()

	router.HandleFunc("/", o.dashboardHandler)
	router.HandleFunc("/ws", o.webSocketHandler)

	http.Handle("/web/", http.StripPrefix("/web/", http.FileServer(http.Dir("./web"))))
	http.Handle("/", router)

	log.Infof("Serving server on %s\n", o.FQDN)

	host, port, _ := net.SplitHostPort(o.FQDN)
	if port == "" {
		port = "80"
	}
	addr := ":" + port

	if o.HttpsKey != "" && o.HttpsCrt != "" {
		key := path.Join("/tmp/certs", o.HttpsKey)
		crt := path.Join("/tmp/certs", o.HttpsCrt)
		err := http.ListenAndServeTLS(addr, crt, key, nil)
		if err != nil {
			log.Fatal("ListenAndServe: ", err)
		}
	} else if o.LetsEntrypt {
		m := &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			Cache:      autocert.DirCache("/tmp/certs"),
			HostPolicy: autocert.HostWhitelist(host),
		}

		s := &http.Server{
			Addr:      addr,
			TLSConfig: m.TLSConfig(),
		}
		go http.ListenAndServe(":80", m.HTTPHandler(nil))
		if err := s.ListenAndServeTLS("", ""); err != nil {
			log.Fatal("ListenAndServe: ", err)
		}
	} else {
		err := http.ListenAndServe(addr, nil)
		if err != nil {
			log.Fatal("ListenAndServe: ", err)
		}
	}
}
