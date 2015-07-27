package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/braintree/manners"
	"github.com/julienschmidt/httprouter"
)

type status struct {
	sync.RWMutex
	ready bool
}

func main() {

	shutdown := make(chan int)

	//create a notification channel to shutdown
	sigChan := make(chan os.Signal, 1)

	//start the main http server for serving traffic
	router := httprouter.New()
	router.GET("/", hello)
	server := manners.NewWithServer(&http.Server{Addr: ":80", Handler: router})
	go func() {
		server.ListenAndServe()
		shutdown <- 1
	}()

	//start the system server for health checks and shutdowns
	s := &status{
		ready: false,
	}

	hRouter := httprouter.New()
	hRouter.GET("/ready", makeReady(s))
	hRouter.GET("/prestop", makePrestop(s))
	go func() {
		http.ListenAndServe(":8080", hRouter)
	}()

	//register for interupt (Ctrl+C) and SIGTERM (docker)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("Shutting down...")
		server.Close()
	}()

	//move server to ready state
	s.Lock()
	s.ready = true
	s.Unlock()

	<-shutdown
}

func hello(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	//simulate 1 seconds work being done
	time.Sleep(1000 * time.Millisecond)
	io.WriteString(w, "Hello world!")
}

func makeReady(s *status) func(http.ResponseWriter, *http.Request, httprouter.Params) {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		s.RLock()
		defer s.RUnlock()
		if s.ready {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
}

func makePrestop(s *status) func(http.ResponseWriter, *http.Request, httprouter.Params) {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		s.Lock()
		s.ready = false
		s.Unlock()

		//can be useful when doing large scaling operations to give readyness probes time to update
		time.Sleep(3 * time.Second)
	}
}
