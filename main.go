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

const VERSION = `1.1.0`

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
	fileBufferPath := flag.String("p", "", "Path to directory for file buffer mode (optional)")
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

	var buffer Buffer              // инициализируем переменню интерфейсом
	buffer = new(LinkedListBuffer) // по-умолчанию устанавливаем буффер связанного списка

	if *fileBufferPath != "" { // если указан путь директории для файлового буфера, то пытаемся использовать файловый буфер
		fileBuffer, err := NewFileBuffer(*fileBufferPath)
		if err != nil {
			os.Exit(1)
		}
		buffer = fileBuffer
	}

	core := NewCore(buffer, *saveFactDomain, *bearer, *saveFactInterval) // создаем контроллер, гдя вся логика. Передаём ему интерфейс буфера, и прочие параметры.

	server := http.Server{Addr: *listen}
	http.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) { // хэндлер для тестирования, если не хотим сразу отправлять данные на сервер назначения
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
