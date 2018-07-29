package tipatch

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"unsafe"

	"github.com/cespare/xxhash"
)

// paddingSize calculates the amount of padding necessary for the Image's page size.
func (img *Image) paddingSize(dataSize int) int {
	pageSize := int(img.pageSize)
	pageMask := pageSize - 1
	pbSize := dataSize & pageMask

	if pbSize == 0 {
		return 0
	}

	return pageSize - pbSize
}

// writePadding writes padding for the image's page size.
func (img *Image) writePadding(out io.Writer, dataSize int) (err error) {
	size := img.paddingSize(dataSize)
	if size == 0 {
		return
	}

	pad := make([]byte, size)
	_, err = out.Write(pad)

	return
}

// checksum computes a checksum corresponding to all the data in the image.
func (img *Image) checksum(hdr *RawImage) uint64 {
	xxh := xxhash.New()

	xxh.Write(img.Kernel)
	xxh.Write(img.Ramdisk)
	xxh.Write(img.Second)
	xxh.Write(img.DeviceTree)

	return xxh.Sum64()
}

// writeHeader writes the Image's header in Android boot format.
func (img *Image) writeHeader(out io.Writer) (err error) {
	var magic [BootMagicSize]byte
	copy(magic[:], BootMagic)

	var board [BootNameSize]byte
	copy(board[:], img.Board)

	var cmdline [BootArgsSize]byte
	var extraCmdline [BootExtraArgsSize]byte

	cmdLen := len(img.Cmdline)
	if cmdLen <= BootArgsSize {
		copy(cmdline[:], img.Cmdline)
	} else if cmdLen <= BootArgsSize+BootExtraArgsSize {
		copy(cmdline[:], img.Cmdline[:BootArgsSize])
		copy(extraCmdline[:], img.Cmdline[BootArgsSize+1:])
	}

	hdr := RawImage{
		Magic: magic,

		KernelSize: uint32(len(img.Kernel)),
		KernelAddr: img.base + img.kernelOffset,

		RamdiskSize: uint32(len(img.Ramdisk)),
		RamdiskAddr: img.base + img.ramdiskOffset,

		SecondSize: uint32(len(img.Second)),
		SecondAddr: img.base + img.secondOffset,

		TagsAddr: img.base + img.tagsOffset,
		PageSize: img.pageSize,
		DtSize:   uint32(len(img.DeviceTree)),

		OSVersion: img.osVersion,

		Board:   board,
		Cmdline: cmdline,

		ID: [32]byte{},

		ExtraCmdline: extraCmdline,
	}

	checksum := img.checksum(&hdr)
	binary.LittleEndian.PutUint64(hdr.ID[:], checksum)

	hdrBytes := *(*[unsafe.Sizeof(hdr)]byte)(unsafe.Pointer(&hdr))
	count, err := out.Write(hdrBytes[:])
	if err != nil {
		return
	}

	err = img.writePadding(out, count)
	if err != nil {
		return
	}

	return
}

// writePaddedSection writes data to the output, then pads it to the page size.
func (img *Image) writePaddedSection(out io.Writer, data []byte) (err error) {
	count, err := out.Write(data)
	if err != nil {
		return
	}

	err = img.writePadding(out, count)
	return
}

// writeData writes the data chunks (ramdisk, kernel, etc) to the output.
func (img *Image) writeData(out io.Writer) (err error) {
	err = img.writePaddedSection(out, img.Kernel)
	if err != nil {
		return
	}

	err = img.writePaddedSection(out, img.Ramdisk)
	if err != nil {
		return
	}

	if len(img.Second) > 0 {
		err = img.writePaddedSection(out, img.Second)
		if err != nil {
			return
		}
	}

	if len(img.DeviceTree) > 0 {
		err = img.writePaddedSection(out, img.DeviceTree)
		if err != nil {
			return
		}
	}

	return
}

// WriteToFd writes all the data of the Image to the provided fd.
func (img *Image) WriteToFd(fd int) (err error) {
	out := os.NewFile(uintptr(fd), "img.img")

	err = img.writeHeader(out)
	if err != nil {
		return
	}

	err = img.writeData(out)
	if err != nil {
		return
	}

	return
}

// DumpBytes dumps the Image data into a byte slice.
func (img *Image) DumpBytes() ([]byte, error) {
	// Calculate size for efficiency
	ps := func(data []byte) int {
		return len(data) + img.paddingSize(len(data))
	}

	var hdr RawImage
	size := int(unsafe.Sizeof(hdr)) + ps(img.Kernel) + ps(img.Ramdisk) + ps(img.Second) + ps(img.DeviceTree)

	buf := bytes.NewBuffer(make([]byte, 0, size))

	err := img.writeHeader(buf)
	if err != nil {
		return nil, err
	}

	err = img.writeData(buf)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
