package main

import (
	"log"
	"net/http"

	"github.com/gilgameshskytrooper/pausepizza/src/kitchen_web_server/utils"
)

func main() {
	hub := newHub()
	go hub.run()
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, utils.Pwd()+"index.html")
	})

	http.HandleFunc("/mess/ws", func(w http.ResponseWriter, r *http.Request) {
		serveClient(hub, w, r)
	})

	if err := http.ListenAndServe(":1004", nil); err != nil {
		log.Fatal("ListenAndServe Error: ", err)
	}

}
