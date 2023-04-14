package main

import (
	"chatroom/global"
	"chatroom/server"
	"fmt"
	"log"
	"net/http"
)

var (
	addr   = ":2023"
	banner = `
    ____              _____
   |    |    |   /\     |
   |    |____|  /  \    | 
   |    |    | /----\   |
   |____|    |/      \  |
    —— WhbudsChatRoom, start on: %s
`
)

func init() {
	global.Init()
}

func main() {
	fmt.Printf(banner, addr)
	server.RegisterHandle()
	log.Fatal(http.ListenAndServe(addr, nil))
}
