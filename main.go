package main

import (
    "fmt"
    "log"
    "net/http"

    "tailscale.com/tsnet"
)

func main() {
    s := new(tsnet.Server)
    s.Hostname = "testhello"
    defer s.Close()
    ln, err := s.Listen("tcp", ":8080")
    if err != nil {
        log.Fatal(err)
    }
    defer ln.Close()

    if _, err := s.LocalClient();  err != nil {
        log.Fatal(err)
    }

    http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, "<html><body><h1>Hello, tailnet!</h1>\n")    
    }))
}

func HelloServer(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "Hello, %s!", r.URL.Path[1:])
}

