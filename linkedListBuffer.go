package main

import "sync"

type LinkedListBuffer struct {
	mu    sync.Mutex
	First *BufferItem
	Last  *BufferItem
}

func (llb *LinkedListBuffer) Add(data BufferItem) {
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

func (llb *LinkedListBuffer) GetFirst() *BufferItem {
	return llb.First
}

func (llb *LinkedListBuffer) GetSize() int {
	count := 0
	data := llb.First
	for {
		if data == nil {
			break
		}
		count++
		data = data.next
	}
	return count
}

func (llb *LinkedListBuffer) RemoveFirst() {
	if llb.First == nil {
		return
	}
	llb.mu.Lock()
	defer llb.mu.Unlock()
	llb.First = llb.First.next // берём ссылку на последующий элемент из первого, и заменяем её первый элемент
	if llb.First == nil {
		llb.Last = nil // если нет первого, то значит нет и последнего
	}
}
