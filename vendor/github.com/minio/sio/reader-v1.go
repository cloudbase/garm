// Copyright (C) 2017 Minio Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sio

import (
	"errors"
	"io"
)

type encReaderV10 struct {
	authEncV10
	src io.Reader

	buffer      packageV10
	recycle     func() // Returns the buffer to the pool
	offset      int
	payloadSize int
	stateErr    error
}

// encryptReaderV10 returns an io.Reader wrapping the given io.Reader.
// The returned io.Reader encrypts everything it reads using DARE 1.0.
func encryptReaderV10(src io.Reader, config *Config) (*encReaderV10, error) {
	ae, err := newAuthEncV10(config)
	if err != nil {
		return nil, err
	}
	buffer, recycle := getBuffer()
	return &encReaderV10{
		authEncV10:  ae,
		src:         src,
		buffer:      buffer,
		recycle:     recycle,
		payloadSize: config.PayloadSize,
	}, nil
}

func (r *encReaderV10) Read(p []byte) (int, error) {
	if r.stateErr != nil {
		return 0, r.stateErr
	}
	var n int
	if r.offset > 0 { // write the buffered package to p
		remaining := r.buffer.Length() - r.offset // remaining encrypted bytes
		if len(p) < remaining {
			n = copy(p, r.buffer[r.offset:r.offset+len(p)])
			r.offset += n
			return n, nil
		}
		n = copy(p, r.buffer[r.offset:r.offset+remaining])
		p = p[remaining:]
		r.offset = 0
	}
	for len(p) >= headerSize+r.payloadSize+tagSize {
		nn, err := io.ReadFull(r.src, p[headerSize:headerSize+r.payloadSize]) // read plaintext from src
		if err != nil && err != io.ErrUnexpectedEOF {
			r.recycle()
			r.stateErr = err
			return n, err // return if reading from src fails or reached EOF
		}
		r.Seal(p, p[headerSize:headerSize+nn])
		n += headerSize + nn + tagSize
		p = p[headerSize+nn+tagSize:]
	}
	if len(p) > 0 {
		nn, err := io.ReadFull(r.src, r.buffer[headerSize:headerSize+r.payloadSize]) // read plaintext from src
		if err != nil && err != io.ErrUnexpectedEOF {
			r.stateErr = err
			r.recycle()
			return n, err // return if reading from src fails or reached EOF
		}
		r.Seal(r.buffer, r.buffer[headerSize:headerSize+nn])
		if length := r.buffer.Length(); length < len(p) {
			r.offset = copy(p, r.buffer[:length])
		} else {
			r.offset = copy(p, r.buffer[:len(p)])
		}
		n += r.offset
	}
	return n, nil
}

type decReaderV10 struct {
	authDecV10
	src io.Reader

	buffer   packageV10
	recycle  func() // Returns the buffer to the pool
	offset   int
	stateErr error
}

// decryptReaderV10 returns an io.Reader wrapping the given io.Reader.
// The returned io.Reader decrypts everything it reads using DARE 1.0.
func decryptReaderV10(src io.Reader, config *Config) (*decReaderV10, error) {
	ad, err := newAuthDecV10(config)
	if err != nil {
		return nil, err
	}
	buffer, recycle := getBuffer()
	return &decReaderV10{
		authDecV10: ad,
		src:        src,
		buffer:     buffer,
		recycle:    recycle,
	}, nil
}

func (r *decReaderV10) Read(p []byte) (n int, err error) {
	if r.stateErr != nil {
		return 0, r.stateErr
	}
	if r.offset > 0 { // write the buffered plaintext to p
		payload := r.buffer.Payload()
		remaining := len(payload) - r.offset // remaining plaintext bytes
		if len(p) < remaining {
			n = copy(p, payload[r.offset:+r.offset+len(p)])
			r.offset += n
			return
		}
		n = copy(p, payload[r.offset:r.offset+remaining])
		p = p[remaining:]
		r.offset = 0
	}
	for len(p) >= maxPayloadSize {
		if err = r.readPackage(r.buffer); err != nil {
			r.stateErr = err
			r.recycle()
			return n, err
		}
		length := len(r.buffer.Payload())
		if err = r.Open(p[:length], r.buffer[:r.buffer.Length()]); err != nil { // notice: buffer.Length() may be smaller than len(buffer)
			r.stateErr = err
			r.recycle()
			return n, err // decryption failed
		}
		p = p[length:]
		n += length
	}
	if len(p) > 0 {
		if err = r.readPackage(r.buffer); err != nil {
			r.stateErr = err
			r.recycle()
			return n, err
		}
		payload := r.buffer.Payload()
		if err = r.Open(payload, r.buffer[:r.buffer.Length()]); err != nil { // notice: buffer.Length() may be smaller than len(buffer)
			r.stateErr = err
			r.recycle()
			return n, err // decryption failed
		}
		if len(payload) < len(p) {
			r.offset = copy(p, payload)
		} else {
			r.offset = copy(p, payload[:len(p)])
		}
		n += r.offset
	}
	return n, nil
}

func (r *decReaderV10) readPackage(dst packageV10) error {
	header := dst.Header()
	_, err := io.ReadFull(r.src, header)
	if err == io.ErrUnexpectedEOF {
		return errInvalidPayloadSize // partial header
	}
	if err != nil {
		return err // reading from src failed or reached EOF
	}

	_, err = io.ReadFull(r.src, dst.Ciphertext())
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		return errInvalidPayloadSize // reading less data than specified by header
	}
	if err != nil {
		return err // reading from src failed or reached EOF
	}
	return nil
}

type decReaderAtV10 struct {
	src io.ReaderAt

	ad authDecV10
}

// decryptReaderAtV10 returns an io.ReaderAt wrapping the given io.ReaderAt.
// The returned io.ReaderAt decrypts everything it reads using DARE 1.0.
func decryptReaderAtV10(src io.ReaderAt, config *Config) (*decReaderAtV10, error) {
	ad, err := newAuthDecV10(config)
	if err != nil {
		return nil, err
	}
	r := &decReaderAtV10{
		ad:  ad,
		src: src,
	}

	return r, nil
}

func (r *decReaderAtV10) ReadAt(p []byte, offset int64) (n int, err error) {
	if offset < 0 {
		return 0, errors.New("sio: DecReaderAt.ReadAt: offset is negative")
	}

	t := offset / int64(maxPayloadSize)
	if t+1 > (1<<32)-1 {
		return 0, errUnexpectedSize
	}

	buffer, recycle := getBuffer()
	defer recycle()

	decReader := decReaderV10{
		authDecV10: r.ad,
		src:        &sectionReader{r.src, t * maxPackageSize},
		buffer:     packageV10(buffer),
		recycle:    recycle,
		offset:     0,
	}
	decReader.SeqNum = uint32(t)
	if k := offset % int64(maxPayloadSize); k > 0 {
		if _, err := io.CopyN(io.Discard, &decReader, k); err != nil {
			return 0, err
		}
	}

	for n < len(p) && err == nil {
		var nn int
		nn, err = (&decReader).Read(p[n:])
		n += nn
	}
	if err == io.EOF && n == len(p) {
		err = nil
	}
	return n, err
}
