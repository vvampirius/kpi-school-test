package main

import (
	_ "embed"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

const VERSION = `1.0`

var (
	ErrorLog = log.New(os.Stderr, `error#`, log.Lshortfile)
	DebugLog = log.New(os.Stdout, `debug#`, log.Lshortfile)

	//go:embed embed/index.html
	indexHtml []byte
)

func helpText() {
	fmt.Println(`bla-bla-bla`)
	flag.PrintDefaults()
}

func main() {
	help := flag.Bool("h", false, "print this help")
	bearer := flag.String("b", "", "Bearer token")
	listen := flag.String("l", ":8080", "listen address")
	saveFactDomain := flag.String("d", "https://development.kpi-drive.ru", "Resend to domain") // Можно использовать http://localhost:8080/test
	ver := flag.Bool("v", false, "Show version")
	saveFactInterval := flag.Int("i", 0, "Interval in milliseconds")
	flag.Parse()

	if *help {
		helpText()
		os.Exit(0)
	}

	if *ver {
		fmt.Println(VERSION)
		os.Exit(0)
	}

	if *bearer == "" {
		fmt.Fprintln(os.Stderr, "bearer token is required")
		os.Exit(1)
	}

	core := NewCore(new(LinkedListBuffer), *saveFactDomain, *bearer, *saveFactInterval)

	server := http.Server{Addr: *listen}
	http.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		DebugLog.Printf("%s %s %s '%s'", r.RemoteAddr, r.Method, r.RequestURI, r.UserAgent())
		DebugLog.Printf("Content-Type: %s", r.Header.Get("Content-Type"))
		DebugLog.Printf("Authorization: %s", r.Header.Get("Authorization"))
		body, _ := io.ReadAll(r.Body)
		fmt.Println(string(body))
	})
	http.HandleFunc("/buffer_size", core.bufferSizeHttpHandler)
	http.HandleFunc("/_api/indicators/get_facts", core.getFactsHttpHandler)
	http.HandleFunc("/favicon.ico", http.NotFound)
	http.HandleFunc("/", core.httpHandler) // хэндлер получения данных и отдачи index.html
	if err := server.ListenAndServe(); err != nil {
		ErrorLog.Fatal(err.Error())
	}
}
