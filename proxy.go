package main

import (
	"fmt"
	"net/http"
	"encoding/json"
	b64 "encoding/base64"
	"time"
)

type requ struct {
	uri string
}

type resp struct {
	Body string
}

type empty struct {}
type semaphore chan empty
func (s semaphore) P(n int) {
	e := empty{}
	for i := 0; i < n; i++ {
		s <- e
	}
}

func (s semaphore) V(n int) {
	for i := 0; i < n; i++ {
		<-s
	}
}

func makeHomeHandler(req chan requ, res chan resp, s semaphore) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		s.P(1)
		request := requ{uri: r.URL.RequestURI()[1:]}
		req <- request
		select {
		case targetResponse := <- res: {
			s.V(1)
			data := targetResponse.Body
			sDec, _ := b64.StdEncoding.DecodeString(data)
			w.Write(sDec)
		}
		case <-time.After(time.Second * 2):
			s.V(1)
		}
	}
}

func makePollHandler(req chan requ, res chan resp) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		var targetResponse resp
		err := decoder.Decode(&targetResponse)
		if err != nil {
			fmt.Printf("Error decoding!")
		}
		if len(targetResponse.Body) > 0 {
			res <- targetResponse
		}
		var op requ
		select {
		case op = <-req:
			op = op
		case <-time.After(time.Second * 10):
			op = requ{uri : ""}
		}
		fmt.Fprintf(w, "{\"url\":\"%s\"}", op.uri)
	}
}

func main() {
	requests := make(chan requ)
	responses := make(chan resp)
	sem := make(semaphore, 1)
	homeHandler := makeHomeHandler(requests, responses, sem)
	http.HandleFunc("/home", homeHandler)
	http.HandleFunc("/web/", homeHandler)
	pollHandler := makePollHandler(requests, responses)
	http.HandleFunc("/poll", pollHandler)
	http.ListenAndServe(":8080", nil)
}

