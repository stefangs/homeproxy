package main // import "github.com/stefangs/homeproxy"

import (
	b64 "encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

type proxyRequest struct {
	Headers []string `json:"headers"`
	Uri     string   `json:"url"`
}

func makeProxyRequest(r *http.Request) proxyRequest {
	headers := make([]string, len(r.Header))
	i := 0
	for k, v := range r.Header {
		headers[i] = k + ": " + v[0]
		i += 1
		log.Println("key:", k, "value:", v)
	}
	return proxyRequest{Uri: r.URL.RequestURI()[1:], Headers: headers}
}

type proxyResponse struct {
	System  string `json:"system"`
	Headers []string `json:"headers"`
	Body    string   `json:"body"`
}

type homeConnection struct {
	requests  chan proxyRequest
	responses chan proxyResponse
	sem       Semaphore
}

func newHomeConnection() *homeConnection {
	return &homeConnection{requests: make(chan proxyRequest), responses: make(chan proxyResponse), sem: make(Semaphore, 1)}
}

func (h *homeConnection) sendRequest(request proxyRequest) (*proxyResponse, error) {
	h.sem.P(1)
	h.requests <- request
	select {
	case targetResponse := <-h.responses:
		{
			h.sem.V(1)
			return &targetResponse, nil
		}
	case <-time.After(time.Second * 2):
		h.sem.V(1)
		return nil, errors.New("Got no response")
	}
}

func (h *homeConnection) pollForRequest(response proxyResponse) (*proxyRequest, error) {
	if len(response.Body) > 0 {
		h.responses <- response
	}
	var nextRequestFromClient proxyRequest
	select {
	case nextRequestFromClient = <-h.requests:
		return &nextRequestFromClient, nil
	case <-time.After(time.Second * 10):
		return nil, errors.New("Timeout")
	}
}

func makeHomeHandler(homes *homeConnections) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		request := makeProxyRequest(r)
		cookie,err := r.Cookie("HomeProxySystem")
		if err != nil {
			fmt.Fprintf(w, "Not logged in\n")
			return
		}
		home, ok := homes.find(cookie.Value)
		if ok {
			targetResponse, err := home.sendRequest(request)
			if err == nil {
				sDec, _ := b64.StdEncoding.DecodeString(targetResponse.Body)
				for _, header := range targetResponse.Headers {
					h := strings.Split(header, ":")
					if !strings.EqualFold(h[0], "Content-Length") {
						w.Header().Add(h[0], h[1])
					}
				}
				w.Write(sDec)
			}
		}
	}
}

func makePollHandler(homes *homeConnections) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		var targetResponse proxyResponse
		err := decoder.Decode(&targetResponse)
		if err != nil {
			fmt.Printf("Error decoding!")
		}
		home := homes.findOrInsert(targetResponse.System)
		op, err := home.pollForRequest(targetResponse)
		if err != nil {
			op = &proxyRequest{Uri: "", Headers: []string{}}
		}
		json.NewEncoder(w).Encode(op)
	}
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name:"HomeProxySystem",Value:"1",MaxAge:20})
}

type homeConnections struct {
	connections map[string]*homeConnection
}

func newHomeConnections() *homeConnections {
	return &homeConnections{make(map[string]*homeConnection)}
}

func (h *homeConnections) findOrInsert(id string) *homeConnection {
	c, ok := h.connections[id];
	if !ok {
		c = newHomeConnection()
		h.connections[id] = c
	}
	return c
}

func (h *homeConnections) find(id string) (*homeConnection, bool) {
	hc, present := h.connections[id];
	return hc, present
}

func main() {
	connections := newHomeConnections()
	homeHandler := makeHomeHandler(connections)
	http.HandleFunc("/home", homeHandler)
	http.HandleFunc("/web/", homeHandler)
	http.HandleFunc("/media/", homeHandler)
	pollHandler := makePollHandler(connections)
	http.HandleFunc("/poll", pollHandler)
	http.HandleFunc("/login", loginHandler)
	http.ListenAndServe(":8080", nil)
}
