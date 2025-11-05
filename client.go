package main

import (
	"bytes"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
)

const BUFFER_SIZE_C = 32 * 1024

func startClient(ip string, uplink string, downlink string, tolink string, token string) {

	listener, err := net.Listen("tcp", ip)
	if err != nil {
		log.Println("Fail to start client: ", err)
		return
	}
	defer listener.Close()

	log.Println("Client started successfully at " + ip)

	for {
		myid := uuid.New().String()[28:]

		conn, err := listener.Accept()
		if err != nil {
			log.Println("Fail to initiate the connection", err)
			continue
		}
		go handleClientUp(conn, uplink, myid, tolink, token)
		go handleClientDown(conn, downlink, myid, token)
	}
}

func handleClientUp(conn net.Conn, uplink string, myid string, tolink string, token string) {

	count := 0
	for {
		buf := make([]byte, BUFFER_SIZE_C)
		n, err := conn.Read(buf)

		if err != nil {
			if err.Error() != "EOF" {
				log.Println("Error reading from tcp request: ", err)
			}
			break
		}

		c := strconv.Itoa(count)
		log.Println("Start up request: ", myid+"/"+c)

		if n > 0 {
			// Create a new POST request
			req, err := http.NewRequest("POST", uplink+"?id="+myid+"&c="+c+"&to="+tolink, bytes.NewBuffer(buf[:n]))
			if err != nil {
				log.Println("Error creating up request:", err)
				break
			}

			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Connection", "close")

			// Perform the request
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				log.Println("Error sending up request: ", err)
				return
			}
			defer resp.Body.Close()

			// Read and print the response
			// responseBody, err := io.ReadAll(resp.Body)
			// if err != nil {
			// 	log.Println("Error reading response body: ", err)
			// }
			// log.Println("Server Response: ", string(responseBody))

			io.ReadAll(resp.Body)
		}

		log.Println("End up request: ", myid+"/"+c)
		count = count + 1
	}
}

func handleClientDown(conn net.Conn, downlink string, myid string, token string) {
	time.Sleep(500 * time.Millisecond)
	log.Println("Start download request: ", myid)
	// Create a new POST request
	req, err := http.NewRequest("GET", downlink+"?id="+myid, nil)
	if err != nil {
		log.Println("Error creating down request: ", err)
		return
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Connection", "close")

	// Perform the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		if err.Error() != "EOF" {
			log.Println("Error sending down request: ", err)
		}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		buf := make([]byte, BUFFER_SIZE_C)

		for {
			n, err := resp.Body.Read(buf)

			if n > 0 {
				conn.Write(buf[:n])
			}

			if err != nil {
				if err.Error() != "EOF" {
					log.Println("Error forwarding down response err: ", err)
				}
				break
			}
		}
	} else {
		log.Println("Down Status Code: ", resp.StatusCode, resp.Status)
	}

	conn.Close()

	log.Println("End down request: ", myid)
}
