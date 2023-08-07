package main

import (
    "crypto/tls"
    "fmt"
    "net/http"

    "tailscale.com/tsnet"
    "tailscale.com/client/tailscale"
)

func main() {
    s := &http.Server{
           TLSConfig: &tls.Config{
                    GetCertificate: tailscale.GetCertificate,
           },
          http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
             fmt.Fprintf(w, "<html><body><h1>Hello from Tailscale!</h1>\n")    
           }),
    }
}


