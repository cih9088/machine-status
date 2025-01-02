package cmd

import (
	"net/http"
	"sync"
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
	machineCaches = make(map[string]string)
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

func (o *ServerOptions) fetchExporters(machineConns map[string]*websocket.Conn, machineCaches map[string]string, isConnOpens map[string]bool) {

  machineChans := make(map[string](chan string))
	for machine, _ := range machineConns {
		machineChans[machine] = make(chan string)
	}

	go func() {
		for {
			for machine, _ := range isConnOpens {
				select {
				case x, ok := <-machineChans[machine]:
					if ok {
						machineCaches[machine] = x
					} else {
						panic("Channel closed!")
					}
				}
			}
		}
	}()

	var wg sync.WaitGroup
	for {
		for machine, isConnOpen := range isConnOpens {
			if isConnOpen {
				wg.Add(1)
				go func(conn *websocket.Conn, cha chan string, isConnOpens map[string]bool, machine string) {
					defer wg.Done()
					err := conn.WriteMessage(1, []byte("fetch"))
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
		wg.Wait()

		time.Sleep(time.Duration(o.Interval) * time.Millisecond)
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
		for _, machine := range o.Machines {
			msg := "<p class='ef9'>Server is offline</p>"
			if isConnOpens[machine] {
				msg = machineCaches[machine]
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

		time.Sleep(time.Duration(o.Interval) * time.Millisecond)
	}
}
