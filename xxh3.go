// Package xxh3 is an extremely fast hash algorithm
package xxh3

import (
	"encoding/binary"
	"math/bits"
	"reflect"
	"unsafe"
)

func XXH_mult32to64(a uint32, b uint64) uint64 { return uint64(a) * uint64(b) }

const KEYSET_DEFAULT_SIZE = 48 /* minimum 32 */

var kKey = []uint32{
	0xb8fe6c39, 0x23a44bbe, 0x7c01812c, 0xf721ad1c,
	0xded46de9, 0x839097db, 0x7240a4a4, 0xb7b3671f,
	0xcb79e64e, 0xccc0e578, 0x825ad07d, 0xccff7221,
	0xb8084674, 0xf743248e, 0xe03590e6, 0x813a264c,
	0x3c2852bb, 0x91c300cb, 0x88d0658b, 0x1b532ea3,
	0x71644897, 0xa20df94e, 0x3819ef46, 0xa9deacd8,
	0xa8fa763f, 0xe39c343f, 0xf9dcbbc7, 0xc70b4f1d,
	0x8a51e04b, 0xcdb45931, 0xc89f7ec9, 0xd9787364,

	0xeac5ac83, 0x34d3ebc3, 0xc581a0ff, 0xfa1363eb,
	0x170ddd51, 0xb7f0da49, 0xd3165526, 0x29d4689e,
	0x2b16be58, 0x7d47a1fc, 0x8ff8b8d1, 0x7ad031ce,
	0x45cb3a8f, 0x95160428, 0xafd7fbca, 0xbb4b407e,
}

func asUint64s(k []uint32) []uint64 {
	hdr := *(*reflect.SliceHeader)(unsafe.Pointer(&k))
	// was uint32, now uint64
	hdr.Len, hdr.Cap = hdr.Len/2, hdr.Cap/2
	return *(*[]uint64)(unsafe.Pointer(&hdr))
}

func XXH3_mul128(ll1, ll2 uint64) uint64 {
	hi, lo := bits.Mul64(ll1, ll2)
	return hi + lo
}

const (
	PRIME64_1 = 11400714785074694791 // 0b1001111000110111011110011011000110000101111010111100101010000111
	PRIME64_2 = 14029467366897019727 // 0b1100001010110010101011100011110100100111110101001110101101001111
	PRIME64_3 = 1609587929392839161  // 0b0001011001010110011001111011000110011110001101110111100111111001
	PRIME64_4 = 9650029242287828579  // 0b1000010111101011110010100111011111000010101100101010111001100011
	PRIME64_5 = 2870177450012600261  // 0b0010011111010100111010110010111100010110010101100110011111000101
)

func XXH_readLE32(ptr []byte) uint32 { return binary.LittleEndian.Uint32(ptr) }
func XXH_readLE64(ptr []byte) uint64 { return binary.LittleEndian.Uint64(ptr) }

func XXH3_avalanche(h64 uint64) uint64 {
	h64 ^= h64 >> 29
	h64 *= PRIME64_3
	h64 ^= h64 >> 32
	return h64
}

func XXH3_len_1to3_64b(data []byte, seed uint64) uint64 {
	key32 := kKey
	c1 := data[0]
	c2 := data[len(data)>>1]
	c3 := data[len(data)-1]
	l1 := uint32(c1) + (uint32(c2) << 8)
	l2 := uint32(len(data)) + (uint32(c3) << 2)
	ll11 := XXH_mult32to64(l1+uint32(seed)+key32[0], uint64(l2)+uint64(key32[1]))
	return XXH3_avalanche(ll11)
}

func XXH3_len_4to8_64b(data []byte, seed uint64) uint64 {
	key32 := kKey

	acc := PRIME64_1 * (uint64(len(data)) + seed)
	l1 := XXH_readLE32(data) + key32[0]
	l2 := XXH_readLE32(data[len(data)-4:]) + key32[1]
	acc += XXH_mult32to64(l1, uint64(l2))
	return XXH3_avalanche(acc)
}

func XXH3_readKey64(ptr []uint64) uint64 { return ptr[0] }

func XXH3_len_9to16_64b(data []byte, seed uint64) uint64 {
	var key64 []uint64 = asUint64s(kKey)

	acc := PRIME64_1 * (uint64(len(data)) + seed)
	ll1 := XXH_readLE64(data) + key64[0]
	ll2 := XXH_readLE64(data[len(data)-8:]) + key64[1]
	acc += XXH3_mul128(ll1, ll2)
	return XXH3_avalanche(acc)
}

func XXH3_len_0to16_64b(data []byte, seed uint64) uint64 {
	if len(data) > 8 {
		return XXH3_len_9to16_64b(data, seed)
	}
	if len(data) >= 4 {
		return XXH3_len_4to8_64b(data, seed)
	}
	if len(data) > 0 {
		return XXH3_len_1to3_64b(data, seed)
	}
	return seed
}

/* ===    Long Keys    === */

const STRIPE_LEN = 64
const STRIPE_ELTS = STRIPE_LEN / 4 // = unsafe.Sizeof(uint32)
const ACC_NB = STRIPE_LEN / 8      // =  unsafe.Sizeof(uint64)

func XXH3_accumulate_512(acc []uint64, data []byte, key []uint32) {
	xacc := acc
	xkey := key

	for i := 0; i < ACC_NB; i++ {
		left := 2 * i
		right := 2*i + 1
		dataLeft := XXH_readLE32(data[4*left:])
		dataRight := XXH_readLE32(data[4*right:])
		xacc[i] += XXH_mult32to64(dataLeft+xkey[left], uint64(dataRight+xkey[right]))
		xacc[i] += uint64(dataLeft) + (uint64(dataRight) << 32)
	}
}

func XXH3_scrambleAcc(acc []uint64, key []uint32) {
	xacc := acc
	xkey := key

	for i := 0; i < ACC_NB; i++ {
		left := 2 * i
		right := 2*i + 1
		xacc[i] ^= xacc[i] >> 47
		p1 := XXH_mult32to64(uint32(xacc[i]), uint64(xkey[left]))
		p2 := XXH_mult32to64(uint32(xacc[i]>>32), uint64(xkey[right]))
		xacc[i] = p1 ^ p2
	}
}

func XXH3_accumulate(acc []uint64, data []byte, key []uint32, nbStripes int) {
	for n := 0; n < nbStripes; n++ {
		XXH3_accumulate_512(acc, data[n*STRIPE_LEN:], key)
		key = key[2:]
	}
}

func XXH3_hashLong(acc []uint64, data []byte) {

	const NB_KEYS = ((KEYSET_DEFAULT_SIZE - STRIPE_ELTS) / 2)

	const block_len = STRIPE_LEN * NB_KEYS
	nb_blocks := len(data) / block_len

	for n := 0; n < nb_blocks; n++ {
		XXH3_accumulate(acc, data[n*block_len:], kKey[:], NB_KEYS)
		XXH3_scrambleAcc(acc, kKey[KEYSET_DEFAULT_SIZE-STRIPE_ELTS:])
	}

	// last partial block
	nbStripes := (len(data) % block_len) / STRIPE_LEN
	XXH3_accumulate(acc, data[nb_blocks*block_len:], kKey[:], nbStripes)

	// last stripe */
	if (len(data) & (STRIPE_LEN - 1)) != 0 {
		p := data[len(data)-STRIPE_LEN:]
		XXH3_accumulate_512(acc, p, kKey[nbStripes*2:])
	}
}

func XXH3_mix16B(data []byte, key []uint32) uint64 {
	key64 := asUint64s(key)

	return XXH3_mul128(
		XXH_readLE64(data)^XXH3_readKey64(key64),
		XXH_readLE64(data[8:])^key64[1])
}

func XXH3_mix2Accs(acc []uint64, key []uint32) uint64 {
	key64 := asUint64s(key)
	return XXH3_mul128(
		acc[0]^XXH3_readKey64(key64),
		acc[1]^key64[1])
}

func XXH3_mergeAccs(acc []uint64, key []uint32, start uint64) uint64 {
	result64 := start

	result64 += XXH3_mix2Accs(acc[0:], key[0:])
	result64 += XXH3_mix2Accs(acc[2:], key[4:])
	result64 += XXH3_mix2Accs(acc[4:], key[8:])
	result64 += XXH3_mix2Accs(acc[6:], key[12:])

	return XXH3_avalanche(result64)
}

func XXH3_hashLong_64b(data []byte, seed uint64) uint64 {
	var acc = []uint64{seed, PRIME64_1, PRIME64_2, PRIME64_3, PRIME64_4, PRIME64_5, -seed, 0}

	XXH3_hashLong(acc, data)

	// converge into final hash
	return XXH3_mergeAccs(acc, kKey, uint64(len(data))*PRIME64_1)
}

func XXH3_64bits_withSeed(data []byte, seed uint64) uint64 {
	p := data
	key := kKey

	if len(data) <= 16 {
		return XXH3_len_0to16_64b(data, seed)
	}

	acc := PRIME64_1 * (uint64(len(data)) + seed)
	len := len(data)
	if len > 32 {
		if len > 64 {
			if len > 96 {
				if len > 128 {
					return XXH3_hashLong_64b(data, seed)
				}

				acc += XXH3_mix16B(p[48:], key[96/4:])
				acc += XXH3_mix16B(p[len-64:], key[112/4:])
			}

			acc += XXH3_mix16B(p[32:], key[64/4:])
			acc += XXH3_mix16B(p[len-48:], key[80/4:])
		}

		acc += XXH3_mix16B(p[16:], key[32/4:])
		acc += XXH3_mix16B(p[len-32:], key[48/4:])

	}

	acc += XXH3_mix16B(p[0:], key[0:])
	acc += XXH3_mix16B(p[len-16:], key[4:])

	return XXH3_avalanche(acc)
}

func XXH3_64bits(data []byte) uint64 {
	return XXH3_64bits_withSeed(data, 0)
}

func Hash(data []byte, seed uint64) uint64 {
	return XXH3_64bits_withSeed(data, seed)
}
