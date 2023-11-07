package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"html/template"

	"github.com/gorilla/csrf"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"tailscale.com/tsnet"
	"tailscale.com/words"
)

var (
	tailsVotes = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "votes",
			Name:      "tails",
			Help:      "This is my counter for tails votes",
		})
	scalesVotes = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "votes",
			Name:      "scales",
			Help:      "This is my counter for scales votes",
		})
)

func main() {
	var hostname = "kubecon-demo"
	ts := &tsnet.Server{Hostname: hostname}

	ln, err := ts.ListenFunnel("tcp", ":443")
	if err != nil {
		log.Fatalf("Error starting tsnet.Server: %v", err)
	}
	localClient, err := ts.LocalClient()
	if err != nil {
		log.Fatal(err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	fmt.Printf("Listening on https://%v\n", ts.CertDomains()[0])

	if lm, err := ts.Listen("tcp", ":9402"); err != nil {
		log.Fatal("Error starting prometheus listener: %v", err)
	} else {
		go func() {
			defer wg.Done()
			http.Handle("/metrics", promhttp.Handler())
			prometheus.MustRegister(tailsVotes)
			prometheus.MustRegister(scalesVotes)
			log.Print("Starting prometheus listener on :9402")

			if err := http.Serve(lm, nil); err != nil {
				log.Fatal("Error serving metrics: %v", err)
			}

			log.Print("Stopping prometheus listener")
		}()
	}

	go func() {
		defer wg.Done()
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

	if err := http.Serve(ln, csrf.Protect(csrfKey())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			processData(r)
		} else if r.Method != "GET" {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		whois, err := localClient.WhoIs(context.Background(), r.RemoteAddr)
		if err != nil {
			http.Error(w, "unable to read user", http.StatusForbidden)
			return
		}
		tmpl := template.Must(template.New("ts").Parse(embeddedTemplate))

		data := struct {
			CSRF      template.HTML
			Tail      Img
			Scale     Img
			LoginName string
		}{
			CSRF:      csrf.TemplateField(r),
			Tail:      getImg(r.Context(), words.Tails()),
			Scale:     getImg(r.Context(), words.Scales()),
			LoginName: whois.UserProfile.LoginName,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		tmpl.Execute(w, data)
	}))); err != nil {
		log.Fatal(err)
	}
	log.Printf("Starting hello server.")

}

func csrfKey() []byte {
	var ret [32]byte
	if _, err := io.ReadFull(rand.Reader, ret[:]); err != nil {
		log.Fatal("not enough randomness to make a CSRF key")
	}
	return ret[:]
}
