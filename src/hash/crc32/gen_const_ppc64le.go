// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ignore

// Generate the constant table associated with the poly used by the
// vpmsumd crc32 algorithm.
//
// go run gen_const_ppc64le.go
//
// generates crc32_table_ppc64le.s

// The following is derived from code written by Anton Blanchard
// <anton@au.ibm.com> found at https://github.com/antonblanchard/crc32-vpmsum.
// The original is dual licensed under GPL and Apache 2.  As the copyright holder
// for the work, IBM has contributed this new work under the golang license.

// This code was written in Go based on the original C implementation.

// This is a tool needed to generate the appropriate constants needed for
// the vpmsum algorithm.  It is included to generate new constant tables if
// new polynomial values are included in the future.

package main

import (
	"bytes"
	"fmt"
	"os"
)

var blocking = 32 * 1024

func reflect_bits(b uint64, nr uint) uint64 {
	var ref uint64

	for bit := uint64(0); bit < uint64(nr); bit++ {
		if (b & uint64(1)) == 1 {
			ref |= (1 << (uint64(nr-1) - bit))
		}
		b = (b >> 1)
	}
	return ref
}

func get_remainder(poly uint64, deg uint, n uint) uint64 {

	rem, _ := xnmodp(n, poly, deg)
	return rem
}

func get_quotient(poly uint64, bits, n uint) uint64 {

	_, div := xnmodp(n, poly, bits)
	return div
}

// xnmodp returns two values, p and div:
// p is the representation of the binary polynomial x**n mod (x ** deg + "poly")
// That is p is the binary representation of the modulus polynomial except for its highest-order term.
// div is the binary representation of the polynomial x**n / (x ** deg + "poly")
func xnmodp(n uint, poly uint64, deg uint) (uint64, uint64) {

	var mod, mask, high, div uint64

	if n < deg {
		div = 0
		return poly, div
	}
	mask = 1<<deg - 1
	poly &= mask
	mod = poly
	div = 1
	deg--
	n--
	for n > deg {
		high = (mod >> deg) & 1
		div = (div << 1) | high
		mod <<= 1
		if high != 0 {
			mod ^= poly
		}
		n--
	}
	return mod & mask, div
}

func main() {
	w := new(bytes.Buffer)

	fmt.Fprintf(w, "// autogenerated: do not edit!\n")
	fmt.Fprintf(w, "// generated from crc32/gen_const_ppc64le.go\n")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "#include \"textflag.h\"\n")

	// These are the polynomials supported in vector now.
	// If adding others, include the polynomial and a name
	// to identify it.

	genCrc32ConstTable(w, 0xedb88320, "IEEE")
	genCrc32ConstTable(w, 0x82f63b78, "Cast")
	genCrc32ConstTable(w, 0xeb31d82e, "Koop")
	b := w.Bytes()

	err := os.WriteFile("crc32_table_ppc64le.s", b, 0666)
	if err != nil {
		fmt.Printf("can't write output: %s\n", err)
	}
}

func genCrc32ConstTable(w *bytes.Buffer, poly uint32, polyid string) {

	ref_poly := reflect_bits(uint64(poly), 32)
	fmt.Fprintf(w, "\n\t/* Reduce %d kbits to 1024 bits */\n", blocking*8)
	j := 0
	for i := (blocking * 8) - 1024; i > 0; i -= 1024 {
		a := reflect_bits(get_remainder(ref_poly, 32, uint(i)), 32) << 1
		b := reflect_bits(get_remainder(ref_poly, 32, uint(i+64)), 32) << 1

		fmt.Fprintf(w, "\t/* x^%d mod p(x)%s, x^%d mod p(x)%s */\n", uint(i+64), "", uint(i), "")
		fmt.Fprintf(w, "DATA ·%sConst+%d(SB)/8,$0x%016x\n", polyid, j*8, b)
		fmt.Fprintf(w, "DATA ·%sConst+%d(SB)/8,$0x%016x\n", polyid, (j+1)*8, a)

		j += 2
		fmt.Fprintf(w, "\n")
	}

	for i := (1024 * 2) - 128; i >= 0; i -= 128 {
		a := reflect_bits(get_remainder(ref_poly, 32, uint(i+32)), 32)
		b := reflect_bits(get_remainder(ref_poly, 32, uint(i+64)), 32)
		c := reflect_bits(get_remainder(ref_poly, 32, uint(i+96)), 32)
		d := reflect_bits(get_remainder(ref_poly, 32, uint(i+128)), 32)

		fmt.Fprintf(w, "\t/* x^%d mod p(x)%s, x^%d mod p(x)%s, x^%d mod p(x)%s, x^%d mod p(x)%s */\n", i+128, "", i+96, "", i+64, "", i+32, "")
		fmt.Fprintf(w, "DATA ·%sConst+%d(SB)/8,$0x%08x%08x\n", polyid, j*8, c, d)
		fmt.Fprintf(w, "DATA ·%sConst+%d(SB)/8,$0x%08x%08x\n", polyid, (j+1)*8, a, b)

		j += 2
		fmt.Fprintf(w, "\n")
	}

	fmt.Fprintf(w, "GLOBL ·%sConst(SB),RODATA,$4336\n", polyid)
	fmt.Fprintf(w, "\n /* Barrett constant m - (4^32)/n */\n")
	fmt.Fprintf(w, "DATA ·%sBarConst(SB)/8,$0x%016x\n", polyid, reflect_bits(get_quotient(ref_poly, 32, 64), 33))
	fmt.Fprintf(w, "DATA ·%sBarConst+8(SB)/8,$0x0000000000000000\n", polyid)
	fmt.Fprintf(w, "DATA ·%sBarConst+16(SB)/8,$0x%016x\n", polyid, reflect_bits((uint64(1)<<32)|ref_poly, 33)) // reflected?
	fmt.Fprintf(w, "DATA ·%sBarConst+24(SB)/8,$0x0000000000000000\n", polyid)
	fmt.Fprintf(w, "GLOBL ·%sBarConst(SB),RODATA,$32\n", polyid)
}
