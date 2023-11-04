package main

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/csrf"
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

	http.Serve(ln, csrf.Protect(csrfKey())(http.HandlerFunc(render)))
	log.Printf("Starting hello server.")
}

func csrfKey() []byte {
	var ret [32]byte
	if _, err := io.ReadFull(rand.Reader, ret[:]); err != nil {
		log.Fatal("not enough randomness to make a CSRF key")
	}
	return ret[:]
}
