package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"time"

	"tailscale.com/tsnet"
)

func main() {
	var hostname = "hello"
	ts := &tsnet.Server{Hostname: hostname}
	if err := ts.Start(); err != nil {
		log.Fatalf("Error starting tsnet.Server: %v", err)
	}
	localClient, err := ts.LocalClient()
	if err != nil {
		log.Fatal(err)
	}

	ln, err := ts.Listen("tcp", ":443")
	ln = tls.NewListener(ln, &tls.Config{
		GetCertificate: localClient.GetCertificate,
	})
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		// wait for tailscale to start before trying to fetch cert names
		for i := 0; i < 60; i++ {
			st, err := localClient.Status(context.Background())
			if err != nil {
				log.Printf("error retrieving tailscale status; retrying: %v", err)
			} else {
				log.Printf("tailscale status: %v", st.BackendState)
				if st.BackendState == "Running" {
					break
				}
			}
			time.Sleep(time.Second)
		}

		l80, err := ts.Listen("tcp", ":80")
		if err != nil {
			log.Fatal(err)
		}
		name, ok := localClient.ExpandSNIName(context.Background(), hostname)
		if !ok {
			log.Fatalf("can't get hostname for https redirect")
		}
		if err := http.Serve(l80, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, fmt.Sprintf("https://%s", name), http.StatusMovedPermanently)
		})); err != nil {
			log.Fatal(err)
		}
	}()

	http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "<html><body><h1>Hello from Tailscale!</h1>\n")
	}))

}
