package main

import (
	"fmt"
	"os"
	"time"
)

const (
	HEART_BEAT_INTERVAL = 90
	VERSION             = "0.1.0"
)

func main() {
	fmt.Println("httpx version: ", VERSION)

	arg_num := len(os.Args)
	if arg_num < 2 {
		fmt.Println("httpx\nhttps://github.com/bingotang1981/httpx")
		os.Exit(0)
	}

	mode := 1

	//第一个参数是服务类型：server/client/udpclient
	stype := os.Args[1]
	if stype == "server" {
		mode = 1
		if arg_num < 4 {
			os.Exit(0)
		}
	} else if stype == "client" {
		mode = 2
		if arg_num < 7 {
			os.Exit(0)
		}
	} else {
		os.Exit(0)
	}

	if mode == 1 {
		// 服务端
		ip := os.Args[2]
		token := os.Args[3]

		go startServer(ip, token)

		for {
			//heartbeat interval is 90 seconds as 100 seconds is the default timeout in cloudflare cdn
			time.Sleep(HEART_BEAT_INTERVAL * time.Second)
			monitorConnHeartbeat()
		}
	} else {
		// 客户端
		ip := os.Args[2]
		uplink := os.Args[3]
		downlink := os.Args[4]
		tolink := os.Args[5]
		token := os.Args[6]

		startClient(ip, uplink, downlink, tolink, token)
	}
}
