package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Success!\n")
	})
	log.Fatal(http.ListenAndServe("127.0.0.1:5555", nil))
}
