package main // import "github.com/stefangs/homeproxy

import (
	"fmt"
	"net/http"
	"encoding/json"
	b64 "encoding/base64"
	"time"
	"log"
	"strings"
)

type proxyRequest struct {
	Headers []string `json:"headers"`
	Uri string `json:"url"`
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
	Headers []string `json:"headers"`
	Body string `json:"body"`
}

func makeHomeHandler(req chan proxyRequest, res chan proxyResponse, s Semaphore) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		s.P(1)
		request := makeProxyRequest(r)
		req <- request
		select {
		case targetResponse := <- res: {
			s.V(1)
			data := targetResponse.Body
			sDec, _ := b64.StdEncoding.DecodeString(data)
			for _,header := range targetResponse.Headers {
				h := strings.Split(header, ":")
				if !strings.EqualFold(h[0], "Content-Length") {
					w.Header().Add(h[0], h[1])
				}
			}
			w.Write(sDec)

		}
		case <-time.After(time.Second * 2):
			s.V(1)
		}
	}
}

func makePollHandler(req chan proxyRequest, res chan proxyResponse) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		var targetResponse proxyResponse
		err := decoder.Decode(&targetResponse)
		if err != nil {
			fmt.Printf("Error decoding!")
		}
		if len(targetResponse.Body) > 0 {
			res <- targetResponse
		}
		var op proxyRequest
		select {
		case op = <-req:
			op = op
		case <-time.After(time.Second * 10):
			op = proxyRequest{Uri : "", Headers: []string{}}
		}
		json.NewEncoder(w).Encode(op)
		//fmt.Fprintf(w, "{\"url\":\"%s\"}", op.Uri)
	}
}

func main() {
	requests := make(chan proxyRequest)
	responses := make(chan proxyResponse)
	sem := make(Semaphore, 1)
	homeHandler := makeHomeHandler(requests, responses, sem)
	http.HandleFunc("/home", homeHandler)
	http.HandleFunc("/web/", homeHandler)
	http.HandleFunc("/media/", homeHandler)
	pollHandler := makePollHandler(requests, responses)
	http.HandleFunc("/poll", pollHandler)
	http.ListenAndServe(":8080", nil)
}

