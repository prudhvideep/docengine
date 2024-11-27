package main

import (
	"log"
	"net/http"

	"github.com/prudhvideep/docengine/server"
)

func main() {
	http.Handle("/", http.FileServer(http.Dir(".")))
	http.HandleFunc("/generate", server.HandleDocGen)

	log.Fatal(http.ListenAndServe(":8080", nil))

}
