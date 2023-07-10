package cmd

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/websocket"
)

type ServerOptions struct {
	Wss         bool
	HttpsKey    string
	HttpsCrt    string
	LetsEntrypt bool
	FQDN        string
	Port        int
	Rootpage    string
	Machines    []string
	Aliases     []string
	Interval    int
	Collapses   []string
}

type IndexPageData struct {
	Machine    string
	Alias      string
	IsCollapse string
}

var (
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
)

func clearSession(response http.ResponseWriter) {
	cookie := &http.Cookie{
		Name:   "session",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	}
	http.SetCookie(response, cookie)
}

func (o *ServerOptions) connectExporters(machineConns map[string]*websocket.Conn, isConnOpens map[string]bool, trigerConnect chan int) {
	for {
		select {
		case <-trigerConnect:
			for _, machine := range o.Machines {
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

func (o *ServerOptions) webSocketHandler(w http.ResponseWriter, r *http.Request) {
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
			log.Debug("Disonnected client from %s with %s", r.RemoteAddr, err)
			break
		} else {
			log.Debug("Received message from client %s", r.RemoteAddr)
		}

		for machine, isConnOpen := range isConnOpens {
			if isConnOpen {
				go func(conn *websocket.Conn, cha chan string, isConnOpens map[string]bool, machine string) {
					err = conn.WriteMessage(mt, message)
					if err != nil {
						isConnOpens[machine] = false
						log.Warnf("Write to exporter machine %s failed: %s", machine, err)
					}
					_, exporter_m, err := conn.ReadMessage()
					if err != nil {
						isConnOpens[machine] = false
						log.Warnf("Read from exporter machine %s failed: %s", machine, err)
					}
					cha <- string(exporter_m)
				}(machineConns[machine], machineChans[machine], isConnOpens, machine)
			}
		}

		for _, machine := range o.Machines {
			msg := "<p class='ef9'>Server is offline</p>"
			if isConnOpens[machine] {
				msg = <-machineChans[machine]
			}
			ws.WriteJSON(struct {
				Machine string
				Data    string
			}{
				Machine: machine,
				Data:    msg,
			})
		}

		if !boolAll(isConnOpens) {
			select {
			case trigerConnect <- 1:
				log.Debug("Try to connect again")
			default:
				log.Debug("Already tring to connect")
			}
		}
	}
}
