package circularBuffer

type CircularBuffer struct {
	values []interface{}
	start  int
	count  int
}

func NewCircularBuffer(capacity int) *CircularBuffer {
	return *CircularBuffer{values: make([]interface{}, 0, capacity), start: 0, count: 0}
}

func (buffer *CircularBuffer) Add(value interface{}) (buffer *CircularBuffer, removed interface{}, wasRemoved bool) {
	wasRemoved := false
	var removedElem interface{}
	size := cap(buffer.values)
	ix := (buffer.start + buffer.count) % size
	if buffer.count < size {
		buffer.count += 1
	} else {
		removedElem = buffer.values[start]
		buffer.start = (buffer.start + 1) % size
		wasRemoved = true
	}
	buffer.values[ix] = value
	return buffer, removedElem, wasRemoved
}

func (buffer *CircularBuffer) Get(index int) interface{} {
	return buffer.values[index%cap(buffer.values)]
}

func (buffer *CircularBuffer) Count() int {
	return buffer.count
}

func (buffer *CircularBuffer) Capacity() int {
	return cap(buffer.values)
}
