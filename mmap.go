// +build linux

package iouring

import (
	"fmt"
	"reflect"
	"runtime"
	"syscall"
	"unsafe"

	iouring_syscall "github.com/iceber/iouring-go/syscall"
)

var (
	uint32Size = uint32(unsafe.Sizeof(uint32(0)))
	sqeSize    = uint32(unsafe.Sizeof(iouring_syscall.SubmissionQueueEntry{}))
)

func mmapIOURing(iour *IOURing) error {
	iour.sq = new(SubmissionQueue)
	iour.cq = new(CompletionQueue)

	if err := mmapSQ(iour); err != nil {
		return err
	}

	if (iour.params.Features & iouring_syscall.IORING_FEAT_SINGLE_MMAP) != 0 {
		iour.cq.ptr = iour.sq.ptr
	}
	if err := mmapCQ(iour); err != nil {
		return err
	}

	if err := mmapSQEs(iour); err != nil {
		return err
	}
	return nil
}

func mmapSQ(iour *IOURing) (err error) {
	sq := iour.sq
	params := iour.params

	sq.size = params.SQOffset.Array + params.SQEntries*uint32Size
	sq.ptr, err = mmap(iour.fd, sq.size, iouring_syscall.IORING_OFF_SQ_RING)
	if err != nil {
		return fmt.Errorf("mmap sq ring: %w", err)
	}

	sq.head = (*uint32)(unsafe.Pointer(sq.ptr + uintptr(params.SQOffset.Head)))
	sq.tail = (*uint32)(unsafe.Pointer(sq.ptr + uintptr(params.SQOffset.Tail)))
	sq.mask = (*uint32)(unsafe.Pointer(sq.ptr + uintptr(params.SQOffset.RingMask)))
	sq.entries = (*uint32)(unsafe.Pointer(sq.ptr + uintptr(params.SQOffset.RingEntries)))
	sq.flags = (*uint32)(unsafe.Pointer(sq.ptr + uintptr(params.SQOffset.Flags)))
	sq.dropped = (*uint32)(unsafe.Pointer(sq.ptr + uintptr(params.SQOffset.Dropped)))

	sq.array = *(*[]uint32)(unsafe.Pointer(&reflect.SliceHeader{
		Data: sq.ptr + uintptr(params.SQOffset.Array),
		Len:  int(params.SQEntries),
		Cap:  int(params.SQEntries),
	}))

	runtime.KeepAlive(sq.ptr)
	return nil
}

func mmapCQ(iour *IOURing) (err error) {
	params := iour.params
	cq := iour.cq

	cq.size = params.CQOffset.Cqes + params.CQEntries*uint32Size
	if cq.ptr == 0 {
		cq.ptr, err = mmap(iour.fd, cq.size, iouring_syscall.IORING_OFF_CQ_RING)
		if err != nil {
			return fmt.Errorf("mmap cq ring: %w", err)
		}
	}

	cq.head = (*uint32)(unsafe.Pointer(cq.ptr + uintptr(params.CQOffset.Head)))
	cq.tail = (*uint32)(unsafe.Pointer(cq.ptr + uintptr(params.CQOffset.Tail)))
	cq.mask = (*uint32)(unsafe.Pointer(cq.ptr + uintptr(params.CQOffset.RingMask)))
	cq.flags = (*uint32)(unsafe.Pointer(cq.ptr + uintptr(params.CQOffset.Flags)))
	cq.overflow = (*uint32)(unsafe.Pointer(cq.ptr + uintptr(params.CQOffset.Overflow)))

	cq.cqes = *(*[]iouring_syscall.CompletionQueueEvent)(
		unsafe.Pointer(&reflect.SliceHeader{
			Data: cq.ptr + uintptr(params.CQOffset.Cqes),
			Len:  int(params.CQEntries),
			Cap:  int(params.CQEntries),
		}))

	runtime.KeepAlive(cq.ptr)
	return nil
}

func mmapSQEs(iour *IOURing) error {
	params := iour.params

	ptr, err := mmap(iour.fd, params.SQEntries*sqeSize, iouring_syscall.IORING_OFF_SQES)
	if err != nil {
		return fmt.Errorf("mmap sqe array: %w", err)
	}

	iour.sq.sqes = *(*[]iouring_syscall.SubmissionQueueEntry)(
		unsafe.Pointer(&reflect.SliceHeader{
			Data: ptr,
			Len:  int(params.SQEntries),
			Cap:  int(params.SQEntries),
		}))

	return nil
}

func mmap(fd int, length uint32, offset uint64) (uintptr, error) {
	ptr, _, errno := syscall.Syscall6(
		syscall.SYS_MMAP,
		0,
		uintptr(length),
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_SHARED|syscall.MAP_POPULATE,
		uintptr(fd),
		uintptr(offset),
	)
	if errno != 0 {
		return 0, errno
	}
	return uintptr(ptr), nil
}
