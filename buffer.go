package main

import (
	"encoding/json"
	"io"
)

type BufferItem struct {
	next        *BufferItem
	ContentType string `json:"content_type"`
	Payload     []byte `json:"-"`
	Uri         string `json:"uri"`
}

func (bufferItem *BufferItem) ReadJSON(r io.Reader) error {
	decoder := json.NewDecoder(r)
	if err := decoder.Decode(bufferItem); err != nil {
		return err
	}
	return nil
}

func (bufferItem *BufferItem) WriteJSON(w io.Writer) error {
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(*bufferItem); err != nil {
		return err
	}
	return nil
}

type Buffer interface {
	Add(item BufferItem)   // добавляем данные в буфер
	GetFirst() *BufferItem // извелакаем первые (наиболе старые данные)
	GetSize() int          // возвращаем размер буффера
	RemoveFirst()          // удаляем первые данные (после передачи по назначениею)
}
