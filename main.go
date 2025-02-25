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

const VERSION = `0.1`

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

type DataElement struct {
	next                *DataElement
	PeriodStart         string `schema:"period_start"`
	PeriodEnd           string `schema:"period_end"`
	PeriodKey           string `schema:"period_key"`
	IndicatorToMoId     int    `schema:"indicator_to_mo_id"`
	IndicatorToMoFactId int    `schema:"indicator_to_mo_fact_id"`
	Value               int    `schema:"value"`
	FactTime            string `schema:"fact_time"`
	IsPlan              int    `schema:"is_plan"`
	AuthUserId          int    `schema:"auth_user_id"`
	Comment             string `schema:"comment"`
}

type Buffer interface {
	Add(DataElement)        // добавляем данные в буфер
	GetFirst() *DataElement // извелакаем первые (наиболе старые данные)
	RemoveFirst()           // удаляем первые данные (после передачи по назначениею)
}

func main() {
	help := flag.Bool("h", false, "print this help")
	bearer := flag.String("b", "", "Bearer token")
	listen := flag.String("l", ":8080", "listen address")
	saveFactUrl := flag.String("s", "https://development.kpi-drive.ru/_api/facts/save_fact", "save_fact URL")
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

	core := NewCore(new(LinkedListBuffer), *saveFactUrl, *bearer, *saveFactInterval)

	server := http.Server{Addr: *listen}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		DebugLog.Printf("%s %s %s '%s'", r.RemoteAddr, r.Method, r.RequestURI, r.UserAgent())
		w.Write(indexHtml)
	})
	http.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		DebugLog.Printf("%s %s %s '%s'", r.RemoteAddr, r.Method, r.RequestURI, r.UserAgent())
		body, _ := io.ReadAll(r.Body)
		fmt.Println(string(body))
	})
	http.HandleFunc("/_api/facts/save_fact", core.httpHandler)
	http.HandleFunc("/_api/indicators/get_facts", core.getFactsHttpHandler)
	http.HandleFunc("/favicon.ico", http.NotFound)
	if err := server.ListenAndServe(); err != nil {
		ErrorLog.Fatal(err.Error())
	}
}
