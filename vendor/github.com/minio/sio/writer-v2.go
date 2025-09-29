// Copyright (C) 2018 Minio Inc.
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
	"io"
)

type encWriterV20 struct {
	authEncV20
	dst io.Writer

	buffer   packageV20
	recycle  func() // Returns the buffer to the pool
	offset   int
	closeErr error
}

// encryptWriterV20 returns an io.WriteCloser wrapping the given io.Writer.
// The returned io.WriteCloser encrypts everything written to it using DARE 2.0
// and writes all encrypted ciphertext as well as the package header and tag
// to the wrapped io.Writer.
//
// The io.WriteCloser must be closed to finalize the encryption successfully.
func encryptWriterV20(dst io.Writer, config *Config) (*encWriterV20, error) {
	ae, err := newAuthEncV20(config)
	if err != nil {
		return nil, err
	}
	buffer, recycle := getBuffer()
	return &encWriterV20{
		authEncV20: ae,
		dst:        dst,
		buffer:     buffer,
		recycle:    recycle,
	}, nil
}

func (w *encWriterV20) Write(p []byte) (n int, err error) {
	if w.finalized {
		// The caller closed the encWriterV20 instance (called encWriterV20.Close()).
		// This is a bug in the calling code - Write after Close is not allowed.
		panic("sio: write to stream after close")
	}
	if w.offset > 0 { // buffer the plaintext data
		remaining := maxPayloadSize - w.offset
		if len(p) <= remaining { // <= is important here to buffer up to 64 KB (inclusively) - see: Close()
			w.offset += copy(w.buffer[headerSize+w.offset:], p)
			return len(p), nil
		}
		n = copy(w.buffer[headerSize+w.offset:], p[:remaining])
		w.Seal(w.buffer, w.buffer[headerSize:headerSize+maxPayloadSize])
		if err = flush(w.dst, w.buffer); err != nil { // write to underlying io.Writer
			w.recycle()
			w.closeErr = err
			return n, err
		}
		p = p[remaining:]
		w.offset = 0
	}
	for len(p) > maxPayloadSize { // > is important here to call Seal (not SealFinal) only if there is at least on package left - see: Close()
		w.Seal(w.buffer, p[:maxPayloadSize])
		if err = flush(w.dst, w.buffer); err != nil { // write to underlying io.Writer
			w.recycle()
			w.closeErr = err
			return n, err
		}
		p = p[maxPayloadSize:]
		n += maxPayloadSize
	}
	if len(p) > 0 {
		w.offset = copy(w.buffer[headerSize:], p)
		n += w.offset
	}
	return n, nil
}

func (w *encWriterV20) Close() (err error) {
	defer w.recycle()

	if w.closeErr != nil {
		if closer, ok := w.dst.(io.Closer); ok {
			closer.Close()
		}
		return w.closeErr
	}

	if w.offset > 0 { // true if at least one Write call happened
		w.SealFinal(w.buffer, w.buffer[headerSize:headerSize+w.offset])
		w.closeErr = flush(w.dst, w.buffer[:headerSize+w.offset+tagSize]) // write to underlying io.Writer
		w.offset = 0
	}
	if closer, ok := w.dst.(io.Closer); ok {
		if err := closer.Close(); w.closeErr == nil {
			w.closeErr = err
		}
	}
	return w.closeErr
}

type decWriterV20 struct {
	authDecV20
	dst io.Writer

	buffer   packageV20
	recycle  func() // Returns the buffer to the pool
	offset   int
	closeErr error
}

// decryptWriterV20 returns an io.WriteCloser wrapping the given io.Writer.
// The returned io.WriteCloser decrypts everything written to it using DARE 2.0
// and writes all decrypted plaintext to the wrapped io.Writer.
//
// The io.WriteCloser must be closed to finalize the decryption successfully.
func decryptWriterV20(dst io.Writer, config *Config) (*decWriterV20, error) {
	ad, err := newAuthDecV20(config)
	if err != nil {
		return nil, err
	}
	buffer, recycle := getBuffer()
	return &decWriterV20{
		authDecV20: ad,
		dst:        dst,
		buffer:     buffer,
		recycle:    recycle,
	}, nil
}

func (w *decWriterV20) Write(p []byte) (n int, err error) {
	if w.offset > 0 { // buffer package
		remaining := headerSize + maxPayloadSize + tagSize - w.offset
		if len(p) < remaining {
			w.offset += copy(w.buffer[w.offset:], p)
			return len(p), nil
		}
		n = copy(w.buffer[w.offset:], p[:remaining])
		plaintext := w.buffer[headerSize : headerSize+maxPayloadSize]
		if err = w.Open(plaintext, w.buffer); err != nil {
			w.recycle()
			w.closeErr = err
			return n, err
		}
		if err = flush(w.dst, plaintext); err != nil { // write to underlying io.Writer
			w.recycle()
			w.closeErr = err
			return n, err
		}
		p = p[remaining:]
		w.offset = 0
	}
	for len(p) >= maxPackageSize {
		plaintext := w.buffer[headerSize : headerSize+maxPayloadSize]
		if err = w.Open(plaintext, p[:maxPackageSize]); err != nil {
			w.recycle()
			w.closeErr = err
			return n, err
		}
		if err = flush(w.dst, plaintext); err != nil { // write to underlying io.Writer
			w.recycle()
			w.closeErr = err
			return n, err
		}
		p = p[maxPackageSize:]
		n += maxPackageSize
	}
	if len(p) > 0 {
		if w.finalized {
			w.recycle()
			w.closeErr = errUnexpectedData
			return n, w.closeErr
		}
		w.offset = copy(w.buffer[:], p)
		n += w.offset
	}
	return n, nil
}

func (w *decWriterV20) Close() (err error) {
	defer w.recycle()

	if w.closeErr != nil {
		if closer, ok := w.dst.(io.Closer); ok {
			closer.Close()
		}
		return w.closeErr
	}

	if w.offset > 0 {
		if w.offset <= headerSize+tagSize { // the payload is always > 0
			w.closeErr = errInvalidPayloadSize
		}
		if w.closeErr == nil {
			w.closeErr = w.Open(w.buffer[headerSize:w.offset-tagSize], w.buffer[:w.offset])
		}
		if w.closeErr == nil {
			w.closeErr = flush(w.dst, w.buffer[headerSize:w.offset-tagSize]) // write to underlying io.Writer
		}
		w.offset = 0
	}

	if closer, ok := w.dst.(io.Closer); ok {
		if err := closer.Close(); w.closeErr == nil {
			w.closeErr = err
		}
	}
	return w.closeErr
}
