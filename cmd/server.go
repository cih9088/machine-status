package cmd

import (
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	fqdn "github.com/Showmax/go-fqdn"
	"github.com/gorilla/mux"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type ServerOptions struct {
	Wss       bool
	HttpsKey  string
	HttpsCrt  string
	Host      string
	Rootpage  string
	Port      int
	Machines  []string
	Debug     bool
	Users     []string
	Passs     []string
	Interval  int
	Collapses []string
}

type Data struct {
	Machine string
	Data    string
}

type IndexPageData struct {
	Machine    string
	IsCollapse string
}

var (
	serverOptions ServerOptions

	cookieHandler = securecookie.New(
		securecookie.GenerateRandomKey(64),
		securecookie.GenerateRandomKey(32))

	dial = websocket.Dialer{
		Subprotocols:    []string{},
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		// wait for 10 seconds
		HandshakeTimeout: 10000 * time.Millisecond,
	}

	router = mux.NewRouter()

	isConnOpens   = make(map[string]bool)
	machineConns  = make(map[string]*websocket.Conn)
	machineChans  = make(map[string](chan string))
	trigerConnect = make(chan int)

	serverCmd = &cobra.Command{
		Use:    "server",
		Short:  "machine-status web service",
		Long:   `machine-status web service`,
		Args:   cobra.NoArgs,
		Run:    serverRun,
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

func setSession(userName string, response http.ResponseWriter) {
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

func clearSession(response http.ResponseWriter) {
	cookie := &http.Cookie{
		Name:   "session",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	}
	http.SetCookie(response, cookie)
}

// login handler
func loginHandler(response http.ResponseWriter, request *http.Request) {
	name := request.FormValue("name")
	pass := request.FormValue("password")
	redirectTarget := serverOptions.Rootpage + "/"
	isValid := false

	for i, _ := range serverOptions.Users {
		if name == serverOptions.Users[i] && pass == serverOptions.Passs[i] {
			// .. check credentials ..
			setSession(name, response)
			redirectTarget = serverOptions.Rootpage + "/dashboard"
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

// logout handler
func logoutHandler(response http.ResponseWriter, request *http.Request) {
	clearSession(response)
	http.Redirect(response, request, serverOptions.Rootpage, 302)
}

func indexPageHandler(response http.ResponseWriter, request *http.Request) {
	indexPage.Execute(response, serverOptions.Rootpage)

	indexPage.Execute(response, struct {
		Page string
		Web  string
	}{
		Page: serverOptions.Rootpage,
		Web:  serverOptions.Rootpage + "/web",
	})
}

func dashboardPageHandler(response http.ResponseWriter, request *http.Request) {
	userName := getUserName(request)
	if stringInSlice(userName, serverOptions.Users) {
		log.Infof("Connected client %s from %s", userName, request.RemoteAddr)

		target := ""
		if serverOptions.Wss {
			target += "wss://"
		} else {
			target += "ws://"
		}
		index := strings.Index(serverOptions.Host, "/")
		if index == -1 {
			if serverOptions.Port == 80 || serverOptions.Port == 443 {
				target += serverOptions.Host + serverOptions.Rootpage + "/ws"
			} else {
				target += serverOptions.Host + ":" + strconv.Itoa(serverOptions.Port) + serverOptions.Rootpage + "/ws"
			}
		} else {
			log.Panic(serverOptions.Host)
		}

		log.Info(target)

		machines := []IndexPageData{}

		for _, machine := range serverOptions.Machines {
			index := strings.Index(machine, ":")
			isCollapse := "checked"
			if stringInSlice(machine, serverOptions.Collapses) {
				isCollapse = ""
			}

			machines = append(machines, IndexPageData{
				Machine:    machine[:index],
				IsCollapse: isCollapse,
			})
		}
		dashboardPage.Execute(response, struct {
			Ws       string
			Web      string
			Interval int
			Machines []IndexPageData
		}{
			Ws:       target,
			Web:      serverOptions.Rootpage + "/web",
			Interval: serverOptions.Interval,
			Machines: machines,
		})
	} else {
		log.Warnf("Cookie is corrupted for user: %s", userName)
		http.Redirect(response, request, serverOptions.Rootpage, 302)
	}
}

func connectExporters(machineConns map[string]*websocket.Conn, isConnOpens map[string]bool, trigerConnect chan int) {
	for {
		select {
		case <-trigerConnect:
			for _, machine := range serverOptions.Machines {
				if !isConnOpens[machine] {
					machineWs, _, err := dial.Dial("ws://"+machine+"/ws", http.Header{})
					if err != nil {
						log.Errorf("Dial error for machine %s: %s:", machine, err)
						continue
					}
					machineConns[machine] = machineWs
					isConnOpens[machine] = true
					log.Infof("%s is connected", machine)
				}
			}
		}
	}
}

func serverWSHandler(w http.ResponseWriter, r *http.Request) {
	// Upgrade initial GET request to a websocket
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}
	// Make sure we close the connection when the function returns
	defer ws.Close()

	for {
		mt, message, err := ws.ReadMessage()
		if err != nil {
			userName := getUserName(r)
			log.Infof("Disonnected client %s from %s with %s", userName, r.RemoteAddr, err)
			break
		}
		log.Debug("Received message from client")

		for machine, isConnOpen := range isConnOpens {
			if isConnOpen {
				go func(conn *websocket.Conn, cha chan string, isConnOpens map[string]bool, machine string) {
					err = conn.WriteMessage(mt, message)
					if err != nil {
						isConnOpens[machine] = false
						log.Warnf("Write to exporter machine %s failed: %s", machine, err)
					}
					mt, message, err = conn.ReadMessage()
					if err != nil {
						isConnOpens[machine] = false
						log.Warnf("Read from exporter machine %s failed: %s", machine, err)
					}
					cha <- string(message)
				}(machineConns[machine], machineChans[machine], isConnOpens, machine)
			}
		}
		log.Debug("Run exporter")

		for _, machine := range serverOptions.Machines {
			msg := "<p class='ef9'>Server is offline</p>"
			if isConnOpens[machine] {
				msg = <-machineChans[machine]
			}
			index := strings.Index(machine, ":")
			machine = machine[:index]
			ws.WriteJSON(Data{
				Machine: machine,
				Data:    msg,
			})
		}

		if ! boolAll(isConnOpens) {
			select {
			case trigerConnect <- 1:
				log.Debug("Try to connect again")
			default:
				log.Debug("Already tring to connect")
			}
		}
	}
}

func init() {
	rootCmd.AddCommand(serverCmd)
	serverCmd.Flags().BoolVar(&serverOptions.Wss, "wss", false,
		"whether use wss for websocket or not")
	serverCmd.Flags().StringVar(&serverOptions.HttpsKey, "https-key", "",
		"location of key to serve https")
	serverCmd.Flags().StringVar(&serverOptions.HttpsCrt, "https-crt", "",
		"location of crt to serve https")
	serverCmd.Flags().StringVar(&serverOptions.Host, "host", fqdn.Get(),
		"fully qualified domain name or ip address")
	serverCmd.Flags().StringVar(&serverOptions.Rootpage, "root", "/",
		"root page for the http server")
	serverCmd.Flags().IntVar(&serverOptions.Port, "port", 80,
		"port to serve")
	serverCmd.Flags().IntVar(&serverOptions.Interval, "interval", 1000,
		"refresh interval in milliseconds")
	serverCmd.Flags().StringSliceVar(&serverOptions.Machines, "machine", []string{},
		"comma seperated exporter machines with port (ex: 'host:9200') ")
	serverCmd.Flags().StringSliceVar(&serverOptions.Users, "user", []string{},
		"comma seperated allowed user list")
	serverCmd.Flags().StringSliceVar(&serverOptions.Passs, "pass", []string{},
		"comma seperated allowed password list that match with user")
	serverCmd.Flags().StringSliceVar(&serverOptions.Collapses, "collapse", []string{},
		"comma seperated exporter machines with port (ex: 'host:9200') to collapse default")
}

// server main method
func serverRun(cmd *cobra.Command, args []string) {
	if rootOptions.Debug {
		log.SetLevel(logrus.DebugLevel)
	}

	serverOptions.Rootpage = strings.Trim(serverOptions.Rootpage, "/")
	if len(serverOptions.Rootpage) != 0 {
		serverOptions.Rootpage = "/" + serverOptions.Rootpage
	}

	for _, machine := range serverOptions.Machines {
		isConnOpens[machine] = false
		machineConns[machine] = &websocket.Conn{}
		machineChans[machine] = make(chan string)
	}

	go connectExporters(machineConns, isConnOpens, trigerConnect)
	trigerConnect <- 1

	router.HandleFunc("/", indexPageHandler)
	router.HandleFunc("/ws", serverWSHandler)
	router.HandleFunc("/dashboard", dashboardPageHandler)

	http.Handle("/web/", http.StripPrefix("/web/", http.FileServer(http.Dir("./web"))))

	router.HandleFunc("/login", loginHandler).Methods("POST")
	router.HandleFunc("/logout", logoutHandler).Methods("POST")
	http.Handle("/", router)

	log.Infof("Serving server on %s with port %d\n", serverOptions.Host, serverOptions.Port)

	err := http.ListenAndServe(":"+strconv.Itoa(serverOptions.Port), nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
	if serverOptions.HttpsCrt != "" && serverOptions.HttpsKey != "" {
		err := http.ListenAndServeTLS(":"+strconv.Itoa(serverOptions.Port),
			serverOptions.HttpsCrt, serverOptions.HttpsKey, nil)
		if err != nil {
			log.Fatal("ListenAndServe: ", err)
		}
	} else {
		err := http.ListenAndServe(":"+strconv.Itoa(serverOptions.Port), nil)
		if err != nil {
			log.Fatal("ListenAndServe: ", err)
		}
	}
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func boolAll(bools map[string]bool) bool {
	is_bool := true
	for _, elem := range bools {
		if !elem {
			is_bool = false
			break
		}
	}
	return is_bool
}

// index page
var indexPage = template.Must(template.New("").Parse(`
<!DOCTYPE html>
<html lang="en">
<head>
	<title>machine-status</title>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
<!--===============================================================================================-->
	<link rel="icon" type="image/png" href="{{.Web}}/images/icons/favicon.ico"/>
<!--===============================================================================================-->
	<link rel="stylesheet" type="text/css" href="{{.Web}}/vendor/bootstrap/css/bootstrap.min.css">
<!--===============================================================================================-->
	<link rel="stylesheet" type="text/css" href="{{.Web}}/fonts/font-awesome-4.7.0/css/font-awesome.min.css">
<!--===============================================================================================-->
	<link rel="stylesheet" type="text/css" href="{{.Web}}/fonts/iconic/css/material-design-iconic-font.min.css">
<!--===============================================================================================-->
	<link rel="stylesheet" type="text/css" href="{{.Web}}/vendor/animate/animate.css">
<!--===============================================================================================-->	
	<link rel="stylesheet" type="text/css" href="{{.Web}}/vendor/css-hamburgers/hamburgers.min.css">
<!--===============================================================================================-->
	<link rel="stylesheet" type="text/css" href="{{.Web}}/vendor/animsition/css/animsition.min.css">
<!--===============================================================================================-->
	<link rel="stylesheet" type="text/css" href="{{.Web}}/vendor/select2/select2.min.css">
<!--===============================================================================================-->
	<link rel="stylesheet" type="text/css" href="{{.Web}}/vendor/daterangepicker/daterangepicker.css">
<!--===============================================================================================-->
	<link rel="stylesheet" type="text/css" href="{{.Web}}/css/util.css">
	<link rel="stylesheet" type="text/css" href="{{.Web}}/css/main.css">
<!--===============================================================================================-->
</head>
<body>
	<div class="limiter">
		<div class="container-login100">
			<div class="wrap-login100">
				<form class="login100-form validate-form" method="post" action="{{.Page}}/login">
					<span class="login100-form-title p-b-26">
						Welcome
					</span>
					<span class="login100-form-title p-b-48">
						<i class="zmdi zmdi-font"></i>
					</span>

						<div class="wrap-input100 validate-input" data-validate = "Valid email is: a@b.c">
							<input class="input100" type="text" name="name" id="name">
							<span class="focus-input100" data-placeholder="ID"></span>
						</div>

						<div class="wrap-input100 validate-input" data-validate="Enter password">
							<span class="btn-show-pass">
								<i class="zmdi zmdi-eye"></i>
							</span>
							<input class="input100" type="password" name="password" id="password">
							<span class="focus-input100" data-placeholder="Password"></span>
						</div>

						<div class="container-login100-form-btn">
							<div class="wrap-login100-form-btn">
								<div class="login100-form-bgbtn"></div>
								<button class="login100-form-btn" type="submit">
									Login
								</button>
							</div>
						</div>

					<div class="text-center p-t-115">
						<a class="txt2" href="https://github.com/cih9088/machine-status">
							machine-status
						</a>
						<span class="txt1">
							by andy
						</span>

					</div>
				</form>
			</div>
		</div>
	</div>
	

	<div id="dropDownSelect1"></div>
	
<!--===============================================================================================-->
	<script src="{{.Web}}/vendor/jquery/jquery-3.2.1.min.js"></script>
<!--===============================================================================================-->
	<script src="{{.Web}}/vendor/animsition/js/animsition.min.js"></script>
<!--===============================================================================================-->
	<script src="{{.Web}}/vendor/bootstrap/js/popper.js"></script>
	<script src="{{.Web}}/vendor/bootstrap/js/bootstrap.min.js"></script>
<!--===============================================================================================-->
	<script src="{{.Web}}/vendor/select2/select2.min.js"></script>
<!--===============================================================================================-->
	<script src="{{.Web}}/vendor/daterangepicker/moment.min.js"></script>
	<script src="{{.Web}}/vendor/daterangepicker/daterangepicker.js"></script>
<!--===============================================================================================-->
	<script src="{{.Web}}/vendor/countdowntime/countdowntime.js"></script>
<!--===============================================================================================-->
	<script src="{{.Web}}/js/main.js"></script>

</body>
</html>
`))

// dashboard page
var dashboardPage = template.Must(template.New("").Parse(`
<!DOCTYPE html>
<html lang="en">
<head>
	<title>machine-status</title>
	<link rel="stylesheet" type="text/css" href="{{.Web}}/css/mystyle.css">
	<link rel="icon" type="image/png" href="{{.Web}}/images/icons/favicon.ico"/>
	<script type="text/javascript">
		window.onload = function () {
			var conn;
			if (window["WebSocket"]) {
				conn = new WebSocket("{{.Ws}}");
				conn.onopen = function (evt) {
					console.log('websockt connection establised', conn);
					conn.send('fetch');
				};
				conn.onerror = function (error) {
					console.log("onerror", error);
				};
				conn.onclose = function (evt) {
					console.log("Connection closed")
					var item = document.getElementById("notice")
					item.innerHTML = "<b>Connection closed</b>";
				};
				conn.onmessage = function (evt) {
					var messages = JSON.parse(evt.data);
					var item = document.getElementById(messages.Machine)
					item.innerHTML = messages.Data;
				};
				window.timer = setInterval( function() { conn.send('fetch'); }, {{.Interval}} );
			} else {
				var item = document.createElement("div");
				item.innerHTML = "<b>Your browser does not support WebSockets.</b>";
			}
		};
	</script>
</head>
<body class="f9 eb15">
<div class="notice f1 b9" id="notice"></div>
<div id="main">
{{range .Machines}}
	<div class="wrap-collabsible">
	<input id="collapsible-{{.Machine}}" class="toggle" type="checkbox" {{.IsCollapse}}>
	<label for="collapsible-{{.Machine}}" class="lbl-toggle">{{.Machine}}</label>
	<div class="collapsible-content">
	<div class="content-inner"><pre class='b9' id="{{.Machine}}"></pre></div></div></div>
{{end}}
</div>
</body>
</html>
`))
