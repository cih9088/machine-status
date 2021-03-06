package cmd

import (
	"html/template"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	"golang.org/x/crypto/acme/autocert"

	fqdn "github.com/Showmax/go-fqdn"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type SimpleOptions struct {
	ServerOptions
	Users []string
	Pwds  []string
}

var (
	serverOptions SimpleOptions

	simpleServerCmd = &cobra.Command{
		Use:    "server-simple",
		Short:  "machine-status simple web service",
		Long:   `machine-status simple web service`,
		Args:   cobra.NoArgs,
		Run:    serverOptions.Run,
		Hidden: false,
	}
)

func getUserName(request *http.Request) (userName string) {
	if cookie, err := request.Cookie("session"); err == nil {
		cookieValue := make(map[string]string)
		if err = cookieHandler.Decode("session", cookie.Value, &cookieValue); err == nil {
			userName = cookieValue["name"]
		}
	}
	return userName
}

func setSessionWithName(userName string, response http.ResponseWriter) {
	value := map[string]string{
		"name": userName,
	}
	if encoded, err := cookieHandler.Encode("session", value); err == nil {
		cookie := &http.Cookie{
			Name:  "session",
			Value: encoded,
			Path:  "/",
		}
		http.SetCookie(response, cookie)
	}
}

func (o *SimpleOptions) loginHandler(response http.ResponseWriter, request *http.Request) {
	name := request.FormValue("name")
	pass := request.FormValue("password")
	redirectTarget := o.Rootpage + "/"
	isValid := false

	for i, _ := range o.Users {
		if name == o.Users[i] && pass == o.Pwds[i] {
			// .. check credentials ..
			setSessionWithName(name, response)
			redirectTarget = o.Rootpage + "/dashboard"
			isValid = true
			break
		}
	}
	if isValid {
		log.Infof("Login success for user %s", name)
	} else {
		log.Warnf("Invalid login attempt: %s %s", name, pass)
	}
	http.Redirect(response, request, redirectTarget, 302)
}

func (o *SimpleOptions) logoutHandler(response http.ResponseWriter, request *http.Request) {
	clearSession(response)
	http.Redirect(response, request, o.Rootpage, 302)
}

func (o *SimpleOptions) indexPageHandler(response http.ResponseWriter, request *http.Request) {
	page, err := template.ParseFiles("web/template/index_simple.html")
	check(err)

	page.Execute(response, struct {
		Page string
		Web  string
	}{
		Page: o.Rootpage,
		Web:  o.Rootpage + "/web",
	})
}

func (o *SimpleOptions) dashboardHandler(response http.ResponseWriter, request *http.Request) {
	userName := getUserName(request)
	if stringInSlice(userName, o.Users) {
		log.Infof("Connected client %s from %s", userName, request.RemoteAddr)

		target := ""
		if o.Wss {
			target += "wss://"
		} else {
			target += "ws://"
		}
		index := strings.Index(o.FQDN, "/")
		if index == -1 {
			if o.Port == 80 || o.Port == 443 {
				target += o.FQDN + o.Rootpage + "/ws"
			} else {
				target += o.FQDN + ":" + strconv.Itoa(o.Port) + o.Rootpage + "/ws"
			}
		} else {
			log.Panic(o.FQDN)
		}

		log.Info(target)

		machines := []IndexPageData{}

		for _, machine := range o.Machines {
			index := strings.Index(machine, ":")
			isCollapse := "checked"
			if stringInSlice(machine, o.Collapses) {
				isCollapse = ""
			}

			machines = append(machines, IndexPageData{
				Machine:    machine[:index],
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
	} else {
		log.Warnf("Cookie is corrupted for user: %s", userName)
		http.Redirect(response, request, o.Rootpage, 302)
	}
}

func init() {
	rootCmd.AddCommand(simpleServerCmd)
	simpleServerCmd.Flags().BoolVar(&serverOptions.Wss, "wss", false,
		"whether use wss for websocket or not")
	simpleServerCmd.Flags().StringVar(&serverOptions.HttpsKey, "https-key", "",
		"location of key to serve https")
	simpleServerCmd.Flags().StringVar(&serverOptions.HttpsCrt, "https-crt", "",
		"location of crt to serve https")
	simpleServerCmd.Flags().BoolVar(&serverOptions.LetsEntrypt, "letsencrypt", false,
		"whether use letsencrypt for https")
	simpleServerCmd.Flags().StringVar(&serverOptions.FQDN, "fqdn", fqdn.Get(),
		"fully qualified domain name or ip address")
	simpleServerCmd.Flags().StringVar(&serverOptions.Rootpage, "root", "/",
		"root page for the http server")
	simpleServerCmd.Flags().IntVar(&serverOptions.Port, "port", 80,
		"port to serve")
	simpleServerCmd.Flags().IntVar(&serverOptions.Interval, "interval", 1000,
		"refresh interval in milliseconds")
	simpleServerCmd.Flags().StringSliceVar(&serverOptions.Machines, "machine", []string{},
		"comma seperated exporter machines with port (ex: 'host:9200') ")
	simpleServerCmd.Flags().StringSliceVar(&serverOptions.Collapses, "collapse", []string{},
		"comma seperated exporter machines with port (ex: 'host:9200') to collapse default")
	simpleServerCmd.Flags().StringSliceVar(&serverOptions.Users, "user", []string{},
		"comma seperated allowed user list")
	simpleServerCmd.Flags().StringSliceVar(&serverOptions.Pwds, "pwd", []string{},
		"comma seperated allowed password list that match with user")
}

// server main method
func (o *SimpleOptions) Run(cmd *cobra.Command, args []string) {
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

	if rootOptions.Debug {
		log.SetLevel(logrus.DebugLevel)
	}

	o.Rootpage = strings.Trim(o.Rootpage, "/")
	if len(o.Rootpage) != 0 {
		o.Rootpage = "/" + o.Rootpage
	}

	for _, machine := range o.Machines {
		isConnOpens[machine] = false
		machineConns[machine] = &websocket.Conn{}
		machineChans[machine] = make(chan string)
	}

	go o.connectExporters(machineConns, isConnOpens, trigerConnect)
	trigerConnect <- 1

	router.HandleFunc("/", o.indexPageHandler)
	router.HandleFunc("/ws", o.webSocketHandler)
	router.HandleFunc("/dashboard", o.dashboardHandler)
	router.HandleFunc("/login", o.loginHandler).Methods("POST")
	router.HandleFunc("/logout", o.logoutHandler).Methods("POST")

	http.Handle("/web/", http.StripPrefix("/web/", http.FileServer(http.Dir("./web"))))
	http.Handle("/", router)

	log.Infof("Serving server on %s with port %d\n", o.FQDN, o.Port)

	addr := ":" + strconv.Itoa(o.Port)

	if o.HttpsKey != "" && o.HttpsCrt != "" {
		key := path.Join("/tmp/certs", o.HttpsKey)
		crt := path.Join("/tmp/certs", o.HttpsCrt)
		err := http.ListenAndServeTLS(addr, key, crt, nil)
		if err != nil {
			log.Fatal("ListenAndServe: ", err)
		}
	} else if o.LetsEntrypt {
		m := &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			Cache:      autocert.DirCache("/tmp/certs"),
			HostPolicy: autocert.HostWhitelist(o.FQDN),
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
