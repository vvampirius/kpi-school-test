package main

import (
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// FileBuffer
//
// В качестве буфера используем файлы. Файлы имеют порядковый номер. %08d.body для тела запроса, %08d.meta для мета-информации в JSON.
type FileBuffer struct {
	mu      sync.Mutex
	Path    string // Путь к директориии с файлами
	FirstId int    // 0 - значит буфер пуст. Файлы начинают нумероваться с 1.
	LastId  int    // 0 - значит буфер пуст
}

func (fb *FileBuffer) Add(item BufferItem) {
	fb.mu.Lock()
	if fb.FirstId == 0 {
		fb.FirstId = 1
	}
	fb.LastId++
	id := fb.LastId
	fb.mu.Unlock()
	if err := fb.SaveItem(item, id); err != nil {
		panic(err) // !!! panic
	}
}

func (fb *FileBuffer) GetBodyPath(id int) string {
	return path.Join(fb.Path, fmt.Sprintf("%08d.body", id))
}

func (fb *FileBuffer) GetMetaPath(id int) string {
	return path.Join(fb.Path, fmt.Sprintf("%08d.meta", id))
}

func (fb *FileBuffer) GetFirst() *BufferItem {
	if fb.FirstId == 0 {
		return nil
	}
	bodyF, err := os.Open(fb.GetBodyPath(fb.FirstId))
	if err != nil {
		ErrorLog.Fatalln(err.Error()) // !!! Fatal
	}
	defer bodyF.Close()
	item := BufferItem{}
	if item.Payload, err = io.ReadAll(bodyF); err != nil {
		ErrorLog.Fatalln(err.Error()) // !!! Fatal
	}
	metaF, err := os.Open(fb.GetMetaPath(fb.FirstId))
	if err != nil {
		ErrorLog.Fatalln(err.Error()) // !!! Fatal
	}
	defer metaF.Close()
	if err = item.ReadJSON(metaF); err != nil {
		ErrorLog.Fatalln(err.Error()) // !!! Fatal
	}
	return &item
}

func (fb *FileBuffer) GetSize() int {
	return fb.LastId - fb.FirstId
}

func (fb *FileBuffer) RemoveFirst() {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	if fb.FirstId == 0 {
		return
	}
	if err := os.Remove(fb.GetBodyPath(fb.FirstId)); err != nil {
		ErrorLog.Fatalln(fmt.Errorf("cannot remove first file: %s", err)) // !!! Fatal
	}
	if err := os.Remove(fb.GetMetaPath(fb.FirstId)); err != nil {
		ErrorLog.Fatalln(fmt.Errorf("cannot remove first file: %s", err)) // !!! Fatal
	}
	fb.FirstId++
	if fb.FirstId > fb.LastId { // Если порядковый номер первого выше, чем последнего, то значит достигли конца буфера, и он пуст. Обнуляемся.
		fb.FirstId = 0
		fb.LastId = 0
	}
}

// ParseId получем порядковый номер из имени файла .body
func (fb *FileBuffer) ParseId(s string) (int, error) {
	s = strings.TrimSuffix(s, ".body")
	parseInt, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		ErrorLog.Printf("%s %s", s, err.Error())
		return 0, err
	}
	return int(parseInt), nil
}

func (fb *FileBuffer) SaveItem(item BufferItem, id int) error {
	bodyF, err := os.OpenFile(fb.GetBodyPath(id), os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		ErrorLog.Println(err.Error())
		return err
	}
	defer bodyF.Close()
	if _, err = bodyF.Write(item.Payload); err != nil {
		ErrorLog.Println(err.Error())
		return err
	}
	metaF, err := os.OpenFile(fb.GetMetaPath(id), os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		ErrorLog.Println(err.Error())
		return err
	}
	defer metaF.Close()
	if err = item.WriteJSON(metaF); err != nil {
		ErrorLog.Println(err.Error())
		return err
	}
	return nil
}

// NewFileBuffer Возвращает файловый буффер, работающий с указанной директорией
func NewFileBuffer(path string) (*FileBuffer, error) {
	fileBuffer := FileBuffer{
		Path: path,
	}
	dirEntries, err := os.ReadDir(path)
	if err != nil {
		ErrorLog.Println(err.Error())
		return nil, err
	}
	bodyFilenames := make([]string, 0)
	for _, dirEntry := range dirEntries {
		if dirEntry.IsDir() {
			continue
		}
		if strings.HasSuffix(dirEntry.Name(), ".body") {
			bodyFilenames = append(bodyFilenames, dirEntry.Name())
		}
	}
	if len(bodyFilenames) == 0 { // Если файлов в директории нет, то выходим. Считаем буффер пустым.
		return &fileBuffer, nil
	}

	// Далее узнаем порядковые номера для первого и последного элемента в буфере.
	sort.Strings(bodyFilenames)
	fileBuffer.FirstId, err = fileBuffer.ParseId(bodyFilenames[0])
	if err != nil {
		return nil, err
	}
	fileBuffer.LastId, err = fileBuffer.ParseId(bodyFilenames[len(bodyFilenames)-1])
	if err != nil {
		return nil, err
	}
	return &fileBuffer, nil
}
