package utils

import (
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/cpu"
)

// Inspired by:
// https://github.com/Workiva/go-datastructures/blob/master/queue/ring.go
// http://www.1024cores.net/home/lock-free-algorithms/queues/bounded-mpmc-queue

const cacheLinePadSize = unsafe.Sizeof(cpu.CacheLinePad{})

type ringNode[T any] struct {
	val T
	pos uint64
}

type ring[T any] []ringNode[T]

type RingBuffer[T any] struct {
	// see: https://arxiv.org/pdf/1012.1824.pdf
	_              cpu.CacheLinePad
	queue          uint64
	_              [cacheLinePadSize - 8]byte
	dequeue        uint64
	_              [cacheLinePadSize - 8]byte
	mask, disposed uint64
	_              [cacheLinePadSize - 16]byte
	ring           ring[T]
}

func NewRingBuffer[T any](size uint64) *RingBuffer[T] {
	if size <= 0 {
		return nil // size must be greater than 0
	}

	pow2size := roundUp(size)

	if pow2size&(pow2size-1) != 0 {
		return nil // size must be a power of 2
	}

	r := &RingBuffer[T]{
		ring: make(ring[T], pow2size),
	}
	for idx := uint64(0); idx < pow2size; idx++ {
		r.ring[idx] = ringNode[T]{pos: idx}
	}
	r.mask = pow2size - 1

	return r
}

func (r *RingBuffer[T]) Enqueue(val T) bool {
	pos := atomic.LoadUint64(&r.queue)
	n := &r.ring[pos&r.mask]
	seq := atomic.LoadUint64(&n.pos)

	if diff := seq - pos; diff == 0 {
		if atomic.CompareAndSwapUint64(&r.queue, pos, pos+1) {
			n.val = val
			atomic.StoreUint64(&n.pos, pos+1)
			return true
		}
	}

	return false
}

func (r *RingBuffer[T]) Dequeue() (val T, ok bool) {
	pos := atomic.LoadUint64(&r.dequeue)
	n := &r.ring[pos&r.mask]
	seq := atomic.LoadUint64(&n.pos)

	if diff := seq - (pos + 1); diff == 0 {
		if atomic.CompareAndSwapUint64(&r.dequeue, pos, pos+1) {
			val = n.val
			ok = true
			atomic.StoreUint64(&n.pos, pos+r.mask+1)
		}
	}

	return
}

func (r *RingBuffer[T]) Full() bool {
	return r.Cap() == r.Len()
}

func (r *RingBuffer[T]) Len() uint64 {
	return atomic.LoadUint64(&r.queue) - atomic.LoadUint64(&r.dequeue)
}

func (r *RingBuffer[T]) Cap() uint64 {
	return uint64(len(r.ring))
}

type RingFillFunc[T any] func() (val T)

func (r *RingBuffer[T]) Fill(filler RingFillFunc[T]) {
	if !r.Full() {
		for r.Enqueue(filler()) {
			// nothing, just fill it.
		}
	}
}

// roundUp to the next power of 2
func roundUp(v uint64) uint64 {
	v--
	v |= v >> 1
	v |= v >> 2
	v |= v >> 4
	v |= v >> 8
	v |= v >> 16
	v |= v >> 32
	v++
	return v
}
