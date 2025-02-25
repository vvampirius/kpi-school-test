package main

import "sync"

type LinkedListBuffer struct {
	mu    sync.Mutex
	First *DataElement
	Last  *DataElement
}

func (llb *LinkedListBuffer) Add(data DataElement) {
	llb.mu.Lock()
	defer llb.mu.Unlock()
	if llb.Last == nil { // в кэше ничего нет
		llb.First = &data
		llb.Last = &data
		return
	}
	llb.Last.next = &data // в последнем элементе делаем ссылку на последующий
	llb.Last = &data      // заменяем ссылку на последни
}

func (llb *LinkedListBuffer) GetFirst() *DataElement {
	return llb.First
}

func (llb *LinkedListBuffer) RemoveFirst() {
	if llb.First == nil {
		return
	}
	llb.mu.Lock()
	defer llb.mu.Unlock()
	llb.First = llb.First.next
	if llb.First == nil {
		llb.Last = nil // если нет первого, то значит нет и последнего
	}
}
