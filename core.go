package main

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/gorilla/schema"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Core struct {
	bufferSizeChan      chan int
	resendLoopStarted   bool // не даёт запуститься нескольким resendLoop()
	Buffer              Buffer
	SaveFactBearerToken string
	SaveFactUrl         string
	SaveFactInterval    time.Duration
}

func (core *Core) bufferSizeHttpHandler(w http.ResponseWriter, r *http.Request) {
	DebugLog.Printf("%s %s %s '%s'", r.RemoteAddr, r.Method, r.RequestURI, r.UserAgent())
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	core.bufferSizeChan = make(chan int, 1)
	core.bufferSizeChan <- core.Buffer.GetSize()
	for size := range core.bufferSizeChan {
		if _, err := fmt.Fprintf(w, "data: %d\n\n", size); err != nil {
			ErrorLog.Println(err.Error())
		}
		w.(http.Flusher).Flush()
	}
}

func (core *Core) getFactsHttpHandler(w http.ResponseWriter, r *http.Request) {
	DebugLog.Printf("%s %s %s '%s'", r.RemoteAddr, r.Method, r.RequestURI, r.UserAgent())
	req, err := http.NewRequest(http.MethodPost, "https://development.kpi-drive.ru/_api/indicators/get_facts", r.Body)
	if err != nil {
		ErrorLog.Println("Error sending request:", err.Error())
		return
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", core.SaveFactBearerToken))
	req.Header.Set("Content-Type", r.Header.Get("Content-Type"))
	client := http.Client{
		Timeout: time.Second * 10,
	}
	resp, err := client.Do(req)
	if err != nil {
		ErrorLog.Println("Error sending request:", err.Error())
		return
	}
	defer resp.Body.Close()
	w.Header().Add("Content-Disposition", "attachment; filename=\"response.json\"")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (core *Core) httpHandler(w http.ResponseWriter, r *http.Request) {
	DebugLog.Printf("%s %s %s '%s'", r.RemoteAddr, r.Method, r.RequestURI, r.UserAgent())
	var data DataElement
	var decoder = schema.NewDecoder()
	contentType := r.Header.Get("Content-Type")
	if contentType == "application/x-www-form-urlencoded" {
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Invalid form", http.StatusBadRequest)
			return
		}
		err = decoder.Decode(&data, r.Form)
		if err != nil {
			ErrorLog.Println(err.Error())
			http.Error(w, "Error parsing form", http.StatusInternalServerError)
			return
		}
	} else if strings.HasPrefix(contentType, "multipart/form-data") {
		err := r.ParseMultipartForm(1024)
		if err != nil {
			ErrorLog.Println(err.Error())
			http.Error(w, "Invalid form", http.StatusBadRequest)
			return
		}
		err = decoder.Decode(&data, r.MultipartForm.Value)
		if err != nil {
			ErrorLog.Println(err.Error())
			http.Error(w, "Error parsing form", http.StatusInternalServerError)
			return
		}
	} else {
		http.Error(w, "Invalid content type", http.StatusBadRequest)
		return
	}
	core.Buffer.Add(data) // добавляем данные в буфер
	core.resendLoop()     // вызываем функцию отправки
	//DebugLog.Printf("Parsed Data: %+v\n", data)
}

func (core *Core) resend(data *DataElement) error {
	DebugLog.Printf("Trying to send: %v", *data)
	encoder := schema.NewEncoder()
	values := url.Values{}
	if err := encoder.Encode(data, values); err != nil {
		ErrorLog.Println(err.Error())
		return err
	}
	body := bytes.NewBufferString(values.Encode())
	req, err := http.NewRequest(http.MethodPost, core.SaveFactUrl, body)
	if err != nil {
		ErrorLog.Println("Error sending request:", err.Error())
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", core.SaveFactBearerToken))
	req.Header.Set("User-Agent", "kpi-school-test (https://github.com/vvampirius/kpi-school-test)")
	client := http.Client{
		Timeout: time.Second * 10,
	}
	resp, err := client.Do(req)
	if err != nil {
		ErrorLog.Println("Error sending request:", err.Error())
		return err
	}
	defer resp.Body.Close()
	DebugLog.Printf("Response status: %d", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		ErrorLog.Printf("Invalid response status code: %d", resp.StatusCode)
		return errors.New("Bad status")
	}
	return nil
}

func (core *Core) resendLoop() {
	if core.resendLoopStarted { // выходим, если уже стартовали
		return
	}
	core.resendLoopStarted = true
	DebugLog.Println("Starting resend loop")
	go func() {
		for {
			// нотифицируем фронтенд о размере буфера
			go func() {
				if core.bufferSizeChan != nil {
					core.bufferSizeChan <- core.Buffer.GetSize()
				}
			}()

			data := core.Buffer.GetFirst()
			if data == nil {
				core.resendLoopStarted = false
				DebugLog.Println("Finishing resend loop")
				return
			}
			if err := core.resend(data); err != nil {
				time.Sleep(time.Second)
				continue // к сожалению, в данной реализации, мы залипнем, если у нас окажется набор данных, который никогда не сможет принять получатель
			}
			core.Buffer.RemoveFirst()
			time.Sleep(core.SaveFactInterval)
		}
	}()
}

func NewCore(buffer Buffer, saveFactUrl string, saveFactBearer string, saveFactInterval int) *Core {
	core := Core{
		Buffer:              buffer,
		SaveFactUrl:         saveFactUrl,
		SaveFactBearerToken: saveFactBearer,
		SaveFactInterval:    time.Duration(saveFactInterval) * time.Millisecond,
	}
	return &core
}
