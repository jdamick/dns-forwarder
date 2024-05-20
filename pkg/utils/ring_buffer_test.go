package utils

import (
	"runtime"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRingBufferIntEnqueueDequeue(t *testing.T) {
	assert := assert.New(t)
	count := 100
	r := NewRingBuffer[int](uint64(count))

	for val := 0; val < count; val++ {
		assert.True(r.Enqueue(val))
	}

	for val := 0; val < count; val++ {
		v, b := r.Dequeue()

		assert.True(b)
		assert.Equal(val, v)
	}
}

func TestRingBufferIntEmptyGet(t *testing.T) {
	r := NewRingBuffer[int](5)
	val, ok := r.Dequeue()
	assert.False(t, ok)
	assert.Equal(t, 0, val)
}

func TestRingBufferSizeZero(t *testing.T) {
	r := NewRingBuffer[int](0)
	assert := assert.New(t)

	assert.Nil(r)
}

func TestRingBufferSizePow2(t *testing.T) {
	r := NewRingBuffer[int](5)
	assert := assert.New(t)
	assert.NotNil(r)
	assert.Equal(uint64(8), r.Cap())
}

func TestRingBufferSingleEnqueueDequeue(t *testing.T) {
	r := NewRingBuffer[int](5)
	assert := assert.New(t)

	assert.Equal(uint64(0), r.Len())
	assert.Equal(uint64(8), r.Cap())
	assert.False(r.Full())

	assert.True(r.Enqueue(99))

	assert.Equal(uint64(1), r.Len())
	assert.Equal(uint64(8), r.Cap())
	assert.False(r.Full())

	v, b := r.Dequeue()
	assert.True(b)
	assert.Equal(99, v)
}

func TestRingBufferFillAndEnqueue(t *testing.T) {
	r := NewRingBuffer[int](8)
	assert := assert.New(t)

	assert.Equal(uint64(0), r.Len())
	assert.Equal(uint64(8), r.Cap())
	assert.False(r.Full())

	for i := 0; i < int(r.Cap()); i++ {
		assert.True(r.Enqueue(i))
	}

	assert.Equal(uint64(8), r.Len())
	assert.Equal(uint64(8), r.Cap())
	assert.True(r.Full())

	assert.False(r.Enqueue(99))

	assert.Equal(uint64(8), r.Len())
	assert.Equal(uint64(8), r.Cap())
	assert.True(r.Full())
}

func TestRingBufferFillAndEmpty(t *testing.T) {
	r := NewRingBuffer[int](8)
	assert := assert.New(t)

	assert.Equal(uint64(0), r.Len())
	assert.Equal(uint64(8), r.Cap())
	assert.False(r.Full())

	for i := 0; i < int(r.Cap()); i++ {
		assert.True(r.Enqueue(i))
	}

	assert.Equal(uint64(8), r.Len())
	assert.Equal(uint64(8), r.Cap())
	assert.True(r.Full())

	for i := 0; i < int(r.Cap()); i++ {
		v, ok := r.Dequeue()
		assert.True(ok)
		assert.Equal(i, v)
	}

	assert.Equal(uint64(0), r.Len())
	assert.Equal(uint64(8), r.Cap())
	assert.False(r.Full())
}

func TestRingBufferFillFunc(t *testing.T) {
	assert := assert.New(t)
	r := NewRingBuffer[byte](8)

	r.Fill(func() byte { return 0xFF })
	assert.True(r.Full())
	val, ok := r.Dequeue()
	assert.True(ok)
	assert.Equal(byte(0xFF), val)
}

func TestRingBufferConccurrent(t *testing.T) {
	assert := assert.New(t)
	r := NewRingBuffer[string](8)

	var wg sync.WaitGroup
	wg.Add(int(r.Cap()))

	strs := []string{}
	for i := 0; i < int(r.Cap()); i++ {
		strs = append(strs, "str: "+strconv.Itoa(i))
	}

	vals := make(map[string]bool)
	for i := 0; i < int(r.Cap()); i++ {
		vals[strs[i]] = true
	}

	go func() {
		for i := 0; i < int(r.Cap()); i++ {
			assert.True(r.Enqueue(strs[i]))
		}
	}()

	go func(vals map[string]bool) {
		received := 0
		for i := 0; received < int(r.Cap()); i++ {
			if val, ok := r.Dequeue(); ok {
				wg.Done()
				received++
				delete(vals, val)
			} else {
				runtime.Gosched()
			}
		}
	}(vals)
	wg.Wait()

	assert.Equal(0, len(vals))
}
