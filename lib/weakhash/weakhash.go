// Copyright (C) 2016 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at http://mozilla.org/MPL/2.0/.

package weakhash

import (
	"bufio"
	"io"
	"os"

	"github.com/chmduquesne/rollinghash/adler32"
)

const (
	Size = 4

	// don't track more hits than this for any given weakhash
	maxWeakhashFinderHits = 10
)

// Find finds all the blocks of the given size within io.Reader that matches
// the hashes provided, and returns a hash -> slice of offsets within reader
// map, that produces the same weak hash.
func Find(ir io.Reader, hashesToFind []uint32, size int) (map[uint32][]int64, error) {
	if ir == nil {
		return nil, nil
	}

	r := bufio.NewReader(ir)
	hf := adler32.New()

	n, err := io.CopyN(hf, r, int64(size))
	if err == io.EOF {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if n != int64(size) {
		return nil, io.ErrShortBuffer
	}

	offsets := make(map[uint32][]int64)
	for _, hashToFind := range hashesToFind {
		offsets[hashToFind] = nil
	}

	var i int64
	var hash uint32
	for {
		hash = hf.Sum32()
		if existing, ok := offsets[hash]; ok && len(existing) < maxWeakhashFinderHits {
			offsets[hash] = append(existing, i)
		}
		i++

		bt, err := r.ReadByte()
		if err == io.EOF {
			break
		} else if err != nil {
			return offsets, err
		}
		hf.Roll(bt)
	}
	return offsets, nil
}

func NewFinder(path string, size int, hashesToFind []uint32) (*Finder, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	offsets, err := Find(file, hashesToFind, size)
	if err != nil {
		file.Close()
		return nil, err
	}

	return &Finder{
		file:    file,
		size:    size,
		offsets: offsets,
	}, nil
}

type Finder struct {
	file    *os.File
	size    int
	offsets map[uint32][]int64
}

// Iterate iterates all available blocks that matches the provided hash, reads
// them into buf, and calls the iterator function. The iterator function should
// return wether it wishes to continue interating.
func (h *Finder) Iterate(hash uint32, buf []byte, iterFunc func(int64) bool) (bool, error) {
	if h == nil || hash == 0 || len(buf) != h.size {
		return false, nil
	}

	for _, offset := range h.offsets[hash] {
		_, err := h.file.ReadAt(buf, offset)
		if err != nil {
			return false, err
		}
		if !iterFunc(offset) {
			return true, nil
		}
	}
	return false, nil
}

// Close releases any resource associated with the finder
func (h *Finder) Close() {
	if h != nil {
		h.file.Close()
	}
}
