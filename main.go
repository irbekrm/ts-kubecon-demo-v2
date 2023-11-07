package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/csrf"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"tailscale.com/client/tailscale"
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
	registry = prometheus.NewRegistry()
)

func init() {
	registry.MustRegister(tailsVotes)
	registry.MustRegister(scalesVotes)
}

type metricsServer struct {
	addr            string
	metricsEndpoint string
}

func (m *metricsServer) Start() error {
	ln, err := net.Listen("tcp", m.addr)
	if err != nil {
		return fmt.Errorf("error listening on: %s: %v", m.addr, err)
	}
	mux := http.NewServeMux()
	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		ErrorHandling: promhttp.HTTPErrorOnError,
	})
	mux.Handle(m.metricsEndpoint, handler)
	srv := http.Server{
		Handler: mux,
	}
	log.Printf("metrics server listening on %s", ln.Addr().String())

	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("error serving metrics: %v", err)
	}
	return nil
}

func main() {
	// metrics
	s := metricsServer{
		addr:            ":9402",
		metricsEndpoint: "/metrics",
	}
	go func() {
		if err := s.Start(); err != nil {
			log.Fatal(err)
		}

	}()

	// tails & scales
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
		fqdn, ok := certDomainForHostname(context.Background(), localClient, hostname)
		if !ok {
			log.Fatalf("could not find a cert for fqdn prefix %s", hostname)
		}
		log.Printf("hostname is %s", hostname)
		if err := http.Serve(l80, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, fmt.Sprintf("https://%s", fqdn), http.StatusMovedPermanently)
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
}

func certDomainForHostname(ctx context.Context, lc *tailscale.LocalClient, hostnamePrefix string) (string, bool) {
	st, err := lc.StatusWithoutPeers(context.Background())
	if err != nil {
		return "", false
	}
	for _, d := range st.CertDomains {
		log.Printf("looking at domain: %s", d)
		if len(d) > len(hostnamePrefix)+1 && strings.HasPrefix(d, hostnamePrefix) {
			log.Printf("selecting %s", d)
			return d, true
		}
	}
	return "", false
}

func csrfKey() []byte {
	var ret [32]byte
	if _, err := io.ReadFull(rand.Reader, ret[:]); err != nil {
		log.Fatal("not enough randomness to make a CSRF key")
	}
	return ret[:]
}
