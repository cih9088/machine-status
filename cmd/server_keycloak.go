package cmd

import (
	"context"
	"errors"
	"html/template"
	"net"
	"net/http"
	"os"
	"path"
	"strings"

	"golang.org/x/crypto/acme/autocert"

	"github.com/Nerzal/gocloak/v8"
	fqdn "github.com/Showmax/go-fqdn"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type KeycloakOptions struct {
	ServerOptions
	KeycloakServer       string
	KeycloakRealm        string
	KeycloakClient       string
	KeycloakClientSecret string
}

var (
	keycloakOptions KeycloakOptions

	keycloakServerCmd = &cobra.Command{
		Use:    "server-keycloak",
		Short:  "machine-status keyclaok authenticated web service",
		Long:   `machine-status keyclaok authenticated web service`,
		Args:   cobra.NoArgs,
		Run:    keycloakOptions.Run,
		Hidden: false,
	}
)

func getSession(request *http.Request) (*gocloak.JWT, error) {
	if cookie, err := request.Cookie("session"); err == nil {
		cookieValue := gocloak.JWT{}
		if err = cookieHandler.Decode("session", cookie.Value, &cookieValue); err == nil {
			return &cookieValue, nil
		}
	}
	return &gocloak.JWT{}, errors.New("error")
}

func setSessionWithToken(userToken *gocloak.JWT, response http.ResponseWriter) {
	if encoded, err := cookieHandler.Encode("session", userToken); err == nil {
		cookie := &http.Cookie{
			Name:  "session",
			Value: encoded,
			Path:  "/",
		}
		http.SetCookie(response, cookie)
	}
}

func (o *KeycloakOptions) refreshToken(userToken *gocloak.JWT) (*gocloak.JWT, *gocloak.UserInfo, error) {
	client := gocloak.NewClient(o.KeycloakServer)
	ctx := context.Background()

	userInfo, err := client.GetUserInfo(
		ctx,
		userToken.AccessToken,
		o.KeycloakRealm,
	)
	if err != nil {
		userToken, err = client.RefreshToken(
			ctx,
			userToken.RefreshToken,
			o.KeycloakClient,
			o.KeycloakClientSecret,
			o.KeycloakRealm,
		)
		if err != nil {
			return nil, nil, err
		} else {
			userInfo, _ = client.GetUserInfo(
				ctx,
				userToken.AccessToken,
				o.KeycloakRealm,
			)
		}
	}
	return userToken, userInfo, err
}

func (o *KeycloakOptions) getUserNameFromSession(response http.ResponseWriter, request *http.Request) (string, error) {

	userToken, err := getSession(request)
	if err != nil {
		return "", err
	}

	client := gocloak.NewClient(o.KeycloakServer)
	ctx := context.Background()

	userInfo, err := client.GetUserInfo(
		ctx,
		userToken.AccessToken,
		o.KeycloakRealm,
	)
	if err != nil {
		log.Infof("Refreshing Token (%s)", err)
		userToken, userInfo, err = o.refreshToken(userToken)
		if err != nil {
			log.Warnf("Token expired (%s)", err)
			clearSession(response)
			return "", err
		}
	}
	return *userInfo.Name, nil
}

func (o *KeycloakOptions) loginHandler(response http.ResponseWriter, request *http.Request) {
	name := request.FormValue("name")
	pass := request.FormValue("password")
	redirectTarget := o.Rootpage + "/"

	client := gocloak.NewClient(o.KeycloakServer)
	ctx := context.Background()

	userToken, err := client.Login(
		ctx,
		o.KeycloakClient,
		o.KeycloakClientSecret,
		o.KeycloakRealm,
		name,
		pass,
	)
	if err != nil {
		log.Warnf("Invalid login attempt for %s (%s)", name, err)
	} else {
		setSessionWithToken(userToken, response)
		redirectTarget = o.Rootpage + "/dashboard"
	}
	http.Redirect(response, request, redirectTarget, 302)
}

// logout handler
func (o *KeycloakOptions) logoutHandler(response http.ResponseWriter, request *http.Request) {

	client := gocloak.NewClient(o.KeycloakServer)
	ctx := context.Background()

	userToken, err := getSession(request)
	log.Info(userToken)
	if err != nil {
		log.Warn(err)
	} else {
		err = client.Logout(
			ctx,
			o.KeycloakClient,
			o.KeycloakClientSecret,
			o.KeycloakRealm,
			userToken.RefreshToken,
		)
		if err != nil {
			log.Warn(err)
		}
	}

	clearSession(response)
	http.Redirect(response, request, o.Rootpage+"/", 302)
}

func (o *KeycloakOptions) indexPageHandler(response http.ResponseWriter, request *http.Request) {
	page, err := template.ParseFiles("web/template/index_keycloak.html")
	check(err)

	page.Execute(response, struct {
		Page           string
		Web            string
		KeycloakServer string
	}{
		Page:           o.Rootpage,
		Web:            o.Rootpage + "/web",
		KeycloakServer: o.KeycloakServer,
	})
}

func (o *KeycloakOptions) dashboardHandler(response http.ResponseWriter, request *http.Request) {
	log.Infof("Connected client  from %s", request.RemoteAddr)

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

	log.Info(target)

	machines := []IndexPageData{}

	for idx := range o.Machines {
		machine := o.Machines[idx]
		alias := o.Aliases[idx]
		isCollapse := "checked"
		if stringInSlice(machine, o.Collapses) {
			isCollapse = ""
		}

		machines = append(machines, IndexPageData{
			Machine:    machine[:index],
			Alias:      alias,
			IsCollapse: isCollapse,
		})
	}

	page, err := template.ParseFiles("web/template/dashboard_keycloak.html")
	check(err)

	page.Execute(response, struct {
		Ws             string
		Web            string
		Interval       int
		Machines       []IndexPageData
		KeycloakServer string
	}{
		Ws:             target,
		Web:            o.Rootpage + "/web",
		Interval:       o.Interval,
		Machines:       machines,
		KeycloakServer: o.KeycloakServer,
	})
}

func init() {
	rootCmd.AddCommand(keycloakServerCmd)
	keycloakServerCmd.Flags().BoolVar(&keycloakOptions.Wss, "wss", false,
		"whether use wss for websocket or not")
	keycloakServerCmd.Flags().StringVar(&keycloakOptions.HttpsKey, "https-key", "",
		"name of key to serve https in /tmp/certs")
	keycloakServerCmd.Flags().StringVar(&keycloakOptions.HttpsCrt, "https-crt", "",
		"name of crt to serve https in /tmp/certs")
	keycloakServerCmd.Flags().BoolVar(&keycloakOptions.LetsEntrypt, "letsencrypt", false,
		"whether use letsencrypt for https")
	keycloakServerCmd.Flags().StringVar(&serverOptions.FQDN, "fqdn", fqdn.Get(),
		"fully qualified domain name or ip address including port. If port is not specified, it assumes '80'. This should be accessable from clinets.")
	keycloakServerCmd.Flags().StringVar(&keycloakOptions.Rootpage, "root", "/",
		"root page for the http server")
	keycloakServerCmd.Flags().IntVar(&keycloakOptions.Interval, "interval", 1000,
		"refresh interval in milliseconds")
	keycloakServerCmd.Flags().StringSliceVar(&keycloakOptions.Machines, "machine", []string{},
		"comma seperated exporter machines with port (ex: 'host:9200' or 'host:9200->alias' with alias) ")
	keycloakServerCmd.Flags().StringSliceVar(&keycloakOptions.Collapses, "collapse", []string{},
		"comma seperated exporter machines with port (ex: 'host:9200') to collapse default")
	keycloakServerCmd.Flags().StringVar(&keycloakOptions.KeycloakServer, "keycloak-server", "",
		"keycloak server")
	keycloakServerCmd.Flags().StringVar(&keycloakOptions.KeycloakRealm, "keycloak-realm", "master",
		"keycloak realm")
	keycloakServerCmd.Flags().StringVar(&keycloakOptions.KeycloakClient, "keycloak-client", "",
		"keycloak client")
	keycloakServerCmd.Flags().StringVar(&keycloakOptions.KeycloakClientSecret, "keycloak-client-secret", "",
		"keycloak client secret")
}

// server main method
func (o *KeycloakOptions) Run(cmd *cobra.Command, args []string) {
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

	log.Infof("Serving server on %s\n", o.FQDN)

	host, port, _ := net.SplitHostPort(o.FQDN)
	if port == "" {
		port = "80"
	}
	addr := ":" + port

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
