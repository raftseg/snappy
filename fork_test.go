// Copyright 2026 The raftseg authors. Use of this source code is governed by
// a BSD-style license that can be found in the LICENSE file.

package snappy

// Tests added by the fork:
//   - differential: on amd64 and arm64 the assembly and the Go code
//     (decodeGo, encodeBlockGo, ...) run side by side on the same input,
//     valid, damaged or arbitrary, and must agree exactly. Elsewhere, or
//     with -tags noasm, both sides are the Go code and the tests check the
//     other properties;
//   - round trips of the block and stream formats, with random write and
//     read sizes, flushes and resets;
//   - hostile input to Decode and Reader: no panics, and no allocation
//     beyond what the input can decode to.

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math/rand"
	"runtime"
	"testing"
)

// maxDiffLen bounds the decoded length the differential decode test
// allocates for.
const maxDiffLen = 1 << 20

// checkDecode decodes src with Decode, and with the assembly and the Go code
// directly, and checks that they agree; that a length Decode refuses up
// front cannot in fact be decoded; and that what decodes re-encodes and
// decodes back to itself.
func checkDecode(t *testing.T, src []byte) {
	got, err := Decode(nil, src)
	dLen, s, herr := decodedLen(src)
	if herr != nil {
		if err == nil {
			t.Fatalf("Decode accepted %x with a bad length header", src)
		}
		return
	}
	if dLen <= maxDiffLen {
		a, b := make([]byte, dLen), make([]byte, dLen)
		ra, rb := decode(a, src[s:]), decodeGo(b, src[s:])
		if ra != rb {
			t.Fatalf("decode(%x): assembly returned %d, Go %d", src, ra, rb)
		}
		if ra == 0 && !bytes.Equal(a, b) {
			t.Fatalf("decode(%x): assembly and Go decoded differently", src)
		}
		if (ra == 0) != (err == nil) {
			t.Fatalf("decode(%x) returned %d but Decode returned %v", src, ra, err)
		}
		if uint64(dLen) > maxDecodedLen(len(src)-s) && rb == 0 {
			t.Fatalf("%x decodes to %d bytes, more than maxDecodedLen(%d) = %d", src, dLen, len(src)-s, maxDecodedLen(len(src)-s))
		}
		if ra == 0 && !bytes.Equal(got, a) {
			t.Fatalf("Decode(%x) differs from decode", src)
		}
	}
	if err != nil {
		return
	}
	if len(got) != dLen {
		t.Fatalf("Decode(%x) returned %d bytes; header says %d", src, len(got), dLen)
	}
	again, err := Decode(nil, Encode(nil, got))
	if err != nil || !bytes.Equal(again, got) {
		t.Fatalf("re-encoding Decode(%x) does not round trip: %v", src, err)
	}
	// Decoding into a large enough dst uses it, and gives the same bytes.
	dst := make([]byte, dLen+7)
	if out, err := Decode(dst, src); err != nil || !bytes.Equal(out, got) || (dLen > 0 && &out[0] != &dst[0]) {
		t.Fatalf("Decode(dst, %x) with a large dst: %v", src, err)
	}
}

// checkEncode encodes src and checks the round trip, the length bounds, and
// that the assembly and Go encoders write the same bytes.
func checkEncode(t *testing.T, src []byte) {
	enc := Encode(nil, src)
	if n := MaxEncodedLen(len(src)); len(enc) > n {
		t.Fatalf("Encode wrote %d bytes, more than MaxEncodedLen(%d) = %d", len(enc), len(src), n)
	}
	if n, err := DecodedLen(enc); err != nil || n != len(src) {
		t.Fatalf("DecodedLen(Encode(%d bytes)) = %d, %v", len(src), n, err)
	}
	dec, err := Decode(nil, enc)
	if err != nil || !bytes.Equal(dec, src) {
		t.Fatalf("Decode(Encode(%x)): %v", src, err)
	}
	if uint64(len(src)) > maxDecodedLen(len(enc)) {
		t.Fatalf("%d bytes encode to %d, but maxDecodedLen(%d) = %d", len(src), len(enc), len(enc), maxDecodedLen(len(enc)))
	}
	for p := src; len(p) >= minNonLiteralBlockSize; {
		block := p
		if len(block) > maxBlockSize {
			block = block[:maxBlockSize]
		}
		p = p[len(block):]
		a := make([]byte, MaxEncodedLen(len(block)))
		b := make([]byte, len(a))
		na, nb := encodeBlock(a, block), encodeBlockGo(b, block)
		if !bytes.Equal(a[:na], b[:nb]) {
			t.Fatalf("encodeBlock of %d bytes: the assembly and Go wrote different bytes", len(block))
		}
	}
}

// checkHelpers compares the assembly and Go versions of emitLiteral,
// emitCopy and extendMatch on parameters drawn from data, within the
// preconditions each documents.
func checkHelpers(t *testing.T, data []byte) {
	if len(data) < 4 {
		return
	}
	n := 1 + int(binary.LittleEndian.Uint16(data))%len(data) // 1 <= len(lit) <= 65536
	lit := bytes.Repeat(data, n/len(data)+1)[:n]
	a, b := make([]byte, n+5), make([]byte, n+5)
	if na, nb := emitLiteral(a, lit), emitLiteralGo(b, lit); na != nb || !bytes.Equal(a, b) {
		t.Fatalf("emitLiteral(%d bytes): assembly %d bytes, Go %d, or different bytes", n, na, nb)
	}
	offset := 1 + int(binary.LittleEndian.Uint16(data[1:]))%65535
	length := 4 + int(binary.LittleEndian.Uint16(data[2:]))%65532
	a, b = make([]byte, 2*length), make([]byte, 2*length)
	if na, nb := emitCopy(a, offset, length), emitCopyGo(b, offset, length); na != nb || !bytes.Equal(a, b) {
		t.Fatalf("emitCopy(%d, %d): assembly %d bytes, Go %d, or different bytes", offset, length, na, nb)
	}
	i := int(data[0]) % len(data)
	j := i + 1 + int(data[1])%(len(data)-i)
	if j > len(data) {
		j = len(data)
	}
	if i < j {
		if x, y := extendMatch(data, i, j), extendMatchGo(data, i, j); x != y {
			t.Fatalf("extendMatch(%x, %d, %d): assembly %d, Go %d", data, i, j, x, y)
		}
	}
}

// checkStream writes data through a Writer (buffered or not) in pieces of
// sizes drawn from rng, with flushes and resets, and reads it back through
// a Reader in random pieces.
func checkStream(t *testing.T, data []byte, rng *rand.Rand) {
	var buf bytes.Buffer
	var w *Writer
	reset := rng.Intn(4) == 0 // a Writer that wrote elsewhere first, then Reset
	var first io.Writer = &buf
	if reset {
		first = io.Discard
	}
	if rng.Intn(2) == 0 {
		w = NewWriter(first)
	} else {
		w = NewBufferedWriter(first)
	}
	if reset {
		w.Write([]byte("discarded"))
		w.Reset(&buf)
	}
	for p := data; len(p) > 0; {
		n := 1 + rng.Intn(len(p))
		if rng.Intn(3) == 0 {
			n = 1 + rng.Intn(min(len(p), 100))
		}
		if _, err := w.Write(p[:n]); err != nil {
			t.Fatal(err)
		}
		p = p[n:]
		if rng.Intn(4) == 0 {
			if err := w.Flush(); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	stream := buf.Bytes()
	r := NewReader(bytes.NewReader(stream))
	var got []byte
	for {
		p := make([]byte, 1+rng.Intn(maxBlockSize+10))
		if rng.Intn(3) == 0 {
			c, err := r.ReadByte()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("ReadByte: %v", err)
			}
			got = append(got, c)
			continue
		}
		n, err := r.Read(p)
		got = append(got, p[:n]...)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("stream of %d bytes read back as %d different bytes", len(data), len(got))
	}
	// A damaged stream must fail cleanly or (a damaged chunk type can make a
	// chunk skippable) read some data; it must never panic.
	if len(stream) > 0 {
		bad := append([]byte(nil), stream...)
		bad[rng.Intn(len(bad))] ^= byte(1 << rng.Intn(8))
		checkReader(t, bad)
	}
}

// checkReader reads an arbitrary stream; only panics and runaway output are
// failures. Each chunk decodes to at most maxBlockSize bytes, so the output
// can never be larger than that many bytes per four bytes of input.
func checkReader(t *testing.T, stream []byte) {
	r := NewReader(bytes.NewReader(stream))
	n, err := io.Copy(io.Discard, r)
	if limit := int64(len(stream)/4+1) * maxBlockSize; n > limit {
		t.Fatalf("a %d-byte stream read as %d bytes", len(stream), n)
	}
	if err == nil {
		return
	}
	r2 := NewReader(nil)
	r2.Reset(bytes.NewReader(stream))
	for {
		if _, err := r2.ReadByte(); err != nil {
			return
		}
	}
}

// corpus is varied data to compress: empty, tiny, random (incompressible),
// repetitive, text-like, and mixtures, across the block size boundary.
func corpus(rng *rand.Rand) [][]byte {
	var out [][]byte
	for _, n := range []int{0, 1, 2, 3, 15, 16, 17, 18, 100, 1000, 65535, 65536, 65537, 140000} {
		random := make([]byte, n)
		rng.Read(random)
		out = append(out, random)
		rep := make([]byte, n)
		for i := range rep {
			rep[i] = "abcabcabd"[i%9]
		}
		out = append(out, rep)
		mixed := make([]byte, n)
		for i := 0; i < n; {
			run := 1 + rng.Intn(200)
			if rng.Intn(2) == 0 && i > 0 {
				off := 1 + rng.Intn(min(i, 70000))
				for k := 0; k < run && i < n; k, i = k+1, i+1 {
					mixed[i] = mixed[i-off]
				}
			} else {
				for k := 0; k < run && i < n; k, i = k+1, i+1 {
					mixed[i] = byte('a' + rng.Intn(4))
				}
			}
		}
		out = append(out, mixed)
	}
	return out
}

func TestDifferentialAndRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	rounds := 30
	if testing.Short() {
		rounds = 5
	}
	for round := 0; round < rounds; round++ {
		for _, src := range corpus(rng) {
			checkEncode(t, src)
			enc := Encode(nil, src)
			checkDecode(t, enc)
			// Damaged encodings are the most realistic hostile input.
			for k := 0; k < 20 && len(enc) > 0; k++ {
				bad := append([]byte(nil), enc...)
				for f := 0; f <= rng.Intn(3); f++ {
					bad[rng.Intn(len(bad))] ^= byte(1 << rng.Intn(8))
				}
				checkDecode(t, bad)
			}
			checkDecode(t, enc[:rng.Intn(len(enc)+1)])
			checkHelpers(t, src)
			if len(src) <= 70000 {
				checkStream(t, src, rng)
			}
		}
	}
}

// TestMaxDecodedLen builds the encodings that decode to the most bytes per
// byte (runs of 64-byte copies after a 1-byte literal, with a short copy at
// the end) and checks they decode, and that maxDecodedLen covers them.
func TestMaxDecodedLen(t *testing.T) {
	for copies := 0; copies < 3000; copies++ {
		for _, tail := range []int{0, 1, 2} {
			body := []byte{0x00, 'x'} // a 1-byte literal
			for i := 0; i < copies; i++ {
				body = append(body, 63<<2|tagCopy2, 1, 0) // 64 bytes at offset 1
			}
			n := 1 + 64*copies
			switch tail {
			case 1: // the most a 1-byte-offset copy writes: 11 bytes
				body = append(body, 7<<2|tagCopy1, 1)
				n += 11
			case 2: // a 4-byte-offset copy of 64 bytes
				body = append(body, 63<<2|tagCopy4, 1, 0, 0, 0)
				n += 64
			}
			src := binary.AppendUvarint(nil, uint64(n))
			src = append(src, body...)
			got, err := Decode(nil, src)
			if err != nil || len(got) != n {
				t.Fatalf("%d copies, tail %d: Decode = %d bytes, %v; want %d", copies, tail, len(got), err, n)
			}
			if uint64(n) > maxDecodedLen(len(body)) {
				t.Fatalf("%d bytes decode to %d, more than maxDecodedLen = %d", len(body), n, maxDecodedLen(len(body)))
			}
			// One byte more than the input can produce is refused.
			src = binary.AppendUvarint(nil, maxDecodedLen(len(body))+1)
			if _, err := Decode(nil, append(src, body...)); !errors.Is(err, ErrCorrupt) {
				t.Fatalf("an impossible length decoded: %v", err)
			}
		}
	}
}

// TestDecodeImpossibleLengthDoesNotAllocate checks the fork's bound: upstream
// allocated whatever the length header claimed, up to 4 GiB, before finding
// the input corrupt.
func TestDecodeImpossibleLengthDoesNotAllocate(t *testing.T) {
	src := binary.AppendUvarint(nil, 1<<30)
	src = append(src, 0x00, 'x', 63<<2|tagCopy2, 1, 0)
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	_, err := Decode(nil, src)
	runtime.ReadMemStats(&after)
	if !errors.Is(err, ErrCorrupt) {
		t.Errorf("Decode = %v; want ErrCorrupt", err)
	}
	if n := after.TotalAlloc - before.TotalAlloc; n > 1<<20 {
		t.Errorf("Decode allocated %d bytes for a %d-byte input claiming 1 GiB", n, len(src))
	}
}

func fuzzSeeds(f *testing.F) {
	rng := rand.New(rand.NewSource(2))
	for _, src := range corpus(rng) {
		if len(src) <= 2000 {
			f.Add(src)
			f.Add(Encode(nil, src))
		}
	}
}

func FuzzDecode(f *testing.F) {
	fuzzSeeds(f)
	f.Fuzz(func(t *testing.T, data []byte) { checkDecode(t, data) })
}

func FuzzEncode(f *testing.F) {
	fuzzSeeds(f)
	f.Fuzz(func(t *testing.T, data []byte) {
		checkEncode(t, data)
		checkHelpers(t, data)
	})
}

func FuzzReader(f *testing.F) {
	rng := rand.New(rand.NewSource(3))
	for _, src := range corpus(rng) {
		if len(src) <= 2000 {
			var buf bytes.Buffer
			w := NewWriter(&buf)
			w.Write(src)
			w.Close()
			f.Add(buf.Bytes())
		}
	}
	f.Fuzz(func(t *testing.T, data []byte) { checkReader(t, data) })
}

func FuzzStream(f *testing.F) {
	fuzzSeeds(f)
	f.Fuzz(func(t *testing.T, data []byte) {
		seed := int64(0)
		if len(data) > 0 {
			seed = int64(data[0])
		}
		checkStream(t, data, rand.New(rand.NewSource(seed)))
	})
}
