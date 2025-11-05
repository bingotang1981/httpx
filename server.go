package main

import (
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

const BUFFER_SIZE_S = 32 * 1024

type twPiece struct {
	tcpConn net.Conn
	uuid    string
	dflag   bool
	t       int64
	c       int
}

var (
	connMap map[string]*twPiece = make(map[string]*twPiece)
	// go的map不是线程安全的 读写冲突就会直接exit
	connMapLock *sync.RWMutex = new(sync.RWMutex)
)

func getConn(uuid string) (*twPiece, bool) {
	connMapLock.Lock()
	defer connMapLock.Unlock()
	conn, haskey := connMap[uuid]
	return conn, haskey
}

func setConn(uuid string, conn *twPiece) {
	connMapLock.Lock()
	defer connMapLock.Unlock()
	connMap[uuid] = conn
}

func deleteConn(uuid string) {
	connMapLock.Lock()
	defer connMapLock.Unlock()
	conn, haskey := connMap[uuid]

	if haskey {
		if conn != nil {
			if conn.tcpConn != nil {
				conn.tcpConn.Close()
			}
		}
		delete(connMap, uuid)
	}
}

// startServer sets up a simple HTTP server to handle the POST request
func startServer(ip string, token string) {
	http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {

		if r.Header.Get("Authorization") != ("Bearer " + token) {
			log.Println("Invalid token: ", r.Header.Get("Authorization"))
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		if r.Method != "POST" {
			log.Println("Invalid method: ", r.Method)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		queryParams, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil {
			log.Println("Fail to parse query: ", err)
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		uuid := queryParams.Get("id")
		if uuid == "" {
			log.Println("No param id in request")
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		toIp := queryParams.Get("to")
		if toIp == "" {
			log.Println("No param to in request")
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		c := queryParams.Get("c")

		if c == "" {
			log.Println("No param c in request")
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		count, err := strconv.Atoi(c)

		if err != nil {
			log.Println("Invalid param c in request", c)
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		log.Println("Start up request: ", uuid+"/"+c)

		conn, hashkey := getConn(uuid)

		if !hashkey {
			if count == 0 {
				tcpconn, err := net.DialTimeout("tcp", toIp, time.Second*5)
				if err != nil {
					log.Println("Server failed to connect to "+toIp, err)
					log.Println("End up request: ", uuid)
					return
				}

				conn = &twPiece{tcpconn, uuid, false, time.Now().Unix(), -1}
				setConn(uuid, conn)

			} else {
				log.Println("Invalid param c in request", c)
				http.Error(w, "Bad request", http.StatusBadRequest)
				return
			}
		}

		if count == conn.c+1 {
			// defer conn.Close()

			buf := make([]byte, BUFFER_SIZE_S)

			for {
				n, err := r.Body.Read(buf)

				if n > 0 {
					conn.t = time.Now().Unix()
					_, err := conn.tcpConn.Write(buf[:n])
					if err != nil {
						log.Println("Error sending up request ", err)
					}
				}

				if err != nil {
					if err.Error() != "EOF" {
						log.Println("Error reading up request", err)
					}

					break
				}
			}

			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte("Upload success"))

			//the count is incremental
			conn.c = count
		} else {
			log.Println("Invalid param c in request", c)
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		log.Println("End up request: ", uuid+"/"+c)

	})

	http.HandleFunc("/download", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != ("Bearer " + token) {
			log.Println("Invalid token: ", r.Header.Get("Authorization"))
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		if r.Method != "GET" {
			log.Println("Invalid method: ", r.Method)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		queryParams, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil {
			log.Println("Fail to parse query: ", err)
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		uuid := queryParams.Get("id")
		if uuid == "" {
			log.Println("No param id in request")
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		// Cast the ResponseWriter to a Flusher to explicitly flush data
		flusher, ok := w.(http.Flusher)
		if !ok {
			log.Println("Streaming unsupported")
			http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
			return
		}

		conn, hashkey := getConn(uuid)
		if !hashkey {
			i := 0
			for {
				i = i + 1
				time.Sleep(1 * time.Second)
				conn, hashkey = getConn(uuid)
				if hashkey {
					break
				}

				if i >= 10 {
					break
				}
			}
		}

		if !hashkey {
			log.Println("No valid uuid: ", uuid)
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		if conn.dflag {
			log.Println("Duplicated uuid: ", uuid)
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		} else {
			conn.dflag = true
		}

		log.Println("Start down request: ", uuid)

		w.Header().Set("Content-Type", "application/octet-stream")

		for {
			buf := make([]byte, BUFFER_SIZE_S)
			n, err := conn.tcpConn.Read(buf)
			if n > 0 {
				conn.t = time.Now().Unix()
				w.Write(buf[:n])
				flusher.Flush()
			}

			if err != nil {
				if err.Error() != "EOF" {
					log.Println("Error sending down response ", err)
				}
				break
			}
		}

		deleteConn(uuid)

		log.Println("End down request: ", uuid)
	})

	log.Println("Server listening on " + ip)

	err := http.ListenAndServe(ip, nil)

	if err != nil {
		log.Println("Fail to start the server")
	}
}

func monitorConnHeartbeat() {
	connMapLock.Lock()
	defer connMapLock.Unlock()

	startTime := time.Now().Unix()
	nowTimeCut := startTime - HEART_BEAT_INTERVAL

	for _, conn := range connMap {
		if conn.t < nowTimeCut {
			delete(connMap, conn.uuid)
		}
	}

	endTime := time.Now().Unix()
	log.Println("Active cons: ", len(connMap), " (", endTime-startTime, "s)")
}
