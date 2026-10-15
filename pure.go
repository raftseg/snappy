// Copyright 2026 The raftseg authors. Use of this source code is governed by
// a BSD-style license that can be found in the LICENSE file.

//go:build (!amd64 && !arm64) || appengine || !gc || noasm

package snappy

// Where there is no assembly (see decode_asm.go and encode_asm.go), the Go
// code decodes and encodes. Fork: upstream built decode_other.go and
// encode_other.go only here, under these names.

func decode(dst, src []byte) int { return decodeGo(dst, src) }

func emitLiteral(dst, lit []byte) int { return emitLiteralGo(dst, lit) }

func emitCopy(dst []byte, offset, length int) int { return emitCopyGo(dst, offset, length) }

func extendMatch(src []byte, i, j int) int { return extendMatchGo(src, i, j) }

func encodeBlock(dst, src []byte) (d int) { return encodeBlockGo(dst, src) }
