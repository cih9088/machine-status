package cmd

import (
	"fmt"
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

type ExporterInfo struct {
	url      string
	isOnline bool
	ws       *websocket.Conn
	status   string
	mu       *sync.RWMutex
}

func NewExporterInfo(url string) *ExporterInfo {
	return &ExporterInfo{url: url, ws: nil, mu: new(sync.RWMutex)}
}

func (i *ExporterInfo) connect() {
	if i.isOnline {
		return
	}

	ws, _, err := dial.Dial("ws://"+i.url+"/ws", http.Header{})
	if err != nil {
		log.Errorf("Dial error for machine %s: %s:", i.url, err)
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	i.ws = ws
	i.isOnline = true
	log.Infof("%s is connected", i.url)
}

func (i *ExporterInfo) fetch() error {
	if !i.isOnline {
		return fmt.Errorf("%s is not online", i.url)
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	err := i.ws.WriteMessage(1, []byte("fetch"))
	if err != nil {
		i.isOnline = false
		_ = i.ws.Close()
		log.Warnf("Write to exporter machine %s failed: %s", i.url, err)
		return err
	}
	_, exporter_m, err := i.ws.ReadMessage()
	if err != nil {
		i.isOnline = false
		_ = i.ws.Close()
		log.Warnf("Read from exporter machine %s failed: %s", i.url, err)
		return err
	}

	i.status = string(exporter_m)

	return nil
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

	exporterInfos = []*ExporterInfo{}
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

func (o *ServerOptions) init() {
	for _, machine := range o.Machines {
		exporterInfos = append(exporterInfos, NewExporterInfo(machine))
	}
}

func (o *ServerOptions) connectAll() {
	wg := new(sync.WaitGroup)
	for _, exporterInfo := range exporterInfos {
		wg.Add(1)
		go func(e *ExporterInfo) {
			defer wg.Done()
			e.connect()
		}(exporterInfo)
	}
	wg.Wait()
}

func (o *ServerOptions) fetchAll() {
	wg := new(sync.WaitGroup)
	for _, exporterInfo := range exporterInfos {
		wg.Add(1)
		go func(e *ExporterInfo) {
			defer wg.Done()
			e.fetch()
		}(exporterInfo)
	}
	wg.Wait()
}

func (o *ServerOptions) connectLoop() {
	for {
		o.connectAll()
		time.Sleep(time.Duration(o.Interval) * time.Millisecond)
	}
}

func (o *ServerOptions) fetchLoop() {
	for {
		o.fetchAll()
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
		for _, exporterInfo := range exporterInfos {
			exporterInfo.mu.RLock()

			status := "<p class='ef9'>Server is offline</p>"
			url := exporterInfo.url
			if exporterInfo.isOnline {
				status = exporterInfo.status
			}

			ws.WriteJSON(struct {
				Machine string
				Data    string
			}{
				Machine: url,
				Data:    status,
			})

			exporterInfo.mu.RUnlock()
		}

		time.Sleep(time.Duration(o.Interval) * time.Millisecond)
	}
}
