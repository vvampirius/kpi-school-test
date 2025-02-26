package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Core struct {
	bufferSizeChan      chan int
	resendLoopStarted   bool // не даёт запуститься нескольким resendLoop()
	Buffer              Buffer
	SaveFactBearerToken string
	SaveFactDomain      string
	SaveFactInterval    time.Duration
}

// bufferSizeHttpHandler
//
// Данный http-хэндлер отправляет фронтенду в EventStream состояние размера буффера. Вызывается из resendLoop().
// Для наглядности, стоит запускать программу с ключом `-i 2000`
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

// getFactsHttpHandler
//
// Данный http-хэндлер нужен просто, что бы из фронтенда получить JSON с результатами попадания данных на конечный
// сервер.
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
	//w.Header().Add("Content-Disposition", "attachment; filename=\"response.json\"")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// httpHandler
//
// Основной http-хэндлер, который получает запрос от клиента, добавляет его в буффер, и вызывает цикл пересылки.
func (core *Core) httpHandler(w http.ResponseWriter, r *http.Request) {
	DebugLog.Printf("%s %s %s '%s'", r.RemoteAddr, r.Method, r.RequestURI, r.UserAgent())
	if r.RequestURI == `/` {
		w.Write(indexHtml)
		return
	}
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		ErrorLog.Println(err.Error())
		return
	}
	bufferItem := BufferItem{
		Uri:         r.RequestURI,
		ContentType: r.Header.Get("Content-Type"),
		Payload:     payload,
	}
	core.Buffer.Add(bufferItem) // добавляем данные в буфер
	core.resendLoop()           // вызываем функцию отправки
}

// resend
//
// Функция, которая делает отправку записи буффера на сервер назначения.
func (core *Core) resend(bufferItem *BufferItem) error {
	resendUrl := core.SaveFactDomain
	if !strings.HasSuffix(resendUrl, "/test") { // Если домен назачения не, к примеру, http://localhost:808/test , то добавляем к не му URI, который был в исходном запросе
		resendUrl = resendUrl + bufferItem.Uri
	}
	DebugLog.Printf("Trying to send '%s' to %s", bufferItem.ContentType, resendUrl)
	body := bytes.NewBuffer(bufferItem.Payload)
	req, err := http.NewRequest(http.MethodPost, resendUrl, body)
	if err != nil {
		ErrorLog.Println("Error sending request:", err.Error())
		return err
	}
	req.Header.Set("Content-Type", bufferItem.ContentType)
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

// resendLoop()
//
// Цикл в go-рутине, который читает FIFO буфер, и вызывает функцию resend()
//
// Можно вызывать многократно - если уже запущен, то вторая копия не запустится.
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

func NewCore(buffer Buffer, saveFactDomain string, saveFactBearer string, saveFactInterval int) *Core {
	core := Core{
		Buffer:              buffer,
		SaveFactDomain:      saveFactDomain,
		SaveFactBearerToken: saveFactBearer,
		SaveFactInterval:    time.Duration(saveFactInterval) * time.Millisecond,
	}
	return &core
}
