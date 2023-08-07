package main

import (
    "crypto/tls"
    "fmt"
    "log"
    "net/http"

    "tailscale.com/tsnet"
)

func main() {
    s := new(tsnet.Server)
    s.Hostname = "testhello"
    defer s.Close()
    ts, err := s.LocalClient() 
    if err != nil {
         log.Fatal(err)
    }

    ln, err := s.Listen("tcp", ":443")
    ln = tls.NewListener(ln, &tls.Config{
              GetCertificate: ts.GetCertificate,
           })
    if err != nil {
        log.Fatal(err)
    }

    http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
             fmt.Fprintf(w, "<html><body><h1>Hello from Tailscale!</h1>\n")    
    }))
}


