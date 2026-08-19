package license

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"errors"
)

// scryptKey is the RFC 7914 scrypt construction specialized only by its
// validated parameter checks. Keeping this small implementation local avoids
// adding a dependency solely for the legacy license derivation.
func scryptKey(password, salt []byte, n, r, p, keyLength int) ([]byte, error) {
	if n <= 1 || n&(n-1) != 0 || r <= 0 || p <= 0 || keyLength < 0 {
		return nil, errors.New("invalid scrypt parameters")
	}
	blockLength := 128 * r
	if blockLength <= 0 || p > int(^uint(0)>>1)/blockLength {
		return nil, errors.New("scrypt parameter overflow")
	}
	first := pbkdf2SHA256(password, salt, p*blockLength)
	for offset := 0; offset < len(first); offset += blockLength {
		smix(first[offset:offset+blockLength], n, r)
	}
	return pbkdf2SHA256(password, first, keyLength), nil
}

func pbkdf2SHA256(password, salt []byte, keyLength int) []byte {
	if keyLength == 0 {
		return []byte{}
	}
	result := make([]byte, 0, keyLength)
	for block := uint32(1); len(result) < keyLength; block++ {
		mac := hmac.New(sha256.New, password)
		_, _ = mac.Write(salt)
		var index [4]byte
		binary.BigEndian.PutUint32(index[:], block)
		_, _ = mac.Write(index[:])
		// scrypt only needs PBKDF2 iteration count one, so U1 is the final
		// block.
		t := mac.Sum(nil)
		result = append(result, t...)
	}
	return result[:keyLength]
}

func smix(block []byte, n, r int) {
	words := 32 * r
	x := make([]uint32, words)
	for i := range x {
		x[i] = binary.LittleEndian.Uint32(block[i*4:])
	}
	v := make([]uint32, n*words)
	for i := 0; i < n; i++ {
		copy(v[i*words:(i+1)*words], x)
		x = blockMix(x, r)
	}
	for i := 0; i < n; i++ {
		j := int(x[(2*r-1)*16] & uint32(n-1))
		for word := range x {
			x[word] ^= v[j*words+word]
		}
		x = blockMix(x, r)
	}
	for i, word := range x {
		binary.LittleEndian.PutUint32(block[i*4:], word)
	}
}

func blockMix(input []uint32, r int) []uint32 {
	words := 32 * r
	x := make([]uint32, 16)
	copy(x, input[words-16:])
	y := make([]uint32, words)
	for i := 0; i < 2*r; i++ {
		for word := 0; word < 16; word++ {
			x[word] ^= input[i*16+word]
		}
		salsa208(x)
		copy(y[i*16:(i+1)*16], x)
	}
	result := make([]uint32, words)
	for i := 0; i < r; i++ {
		copy(result[i*16:(i+1)*16], y[i*32:(i*2+1)*16])
		copy(result[(r+i)*16:(r+i+1)*16], y[(i*2+1)*16:(i*2+2)*16])
	}
	return result
}

func salsa208(x []uint32) {
	initial := append([]uint32(nil), x...)
	for round := 0; round < 8; round += 2 {
		// Column round.
		x[4] ^= rotateLeft(x[0]+x[12], 7)
		x[8] ^= rotateLeft(x[4]+x[0], 9)
		x[12] ^= rotateLeft(x[8]+x[4], 13)
		x[0] ^= rotateLeft(x[12]+x[8], 18)
		x[9] ^= rotateLeft(x[5]+x[1], 7)
		x[13] ^= rotateLeft(x[9]+x[5], 9)
		x[1] ^= rotateLeft(x[13]+x[9], 13)
		x[5] ^= rotateLeft(x[1]+x[13], 18)
		x[14] ^= rotateLeft(x[10]+x[6], 7)
		x[2] ^= rotateLeft(x[14]+x[10], 9)
		x[6] ^= rotateLeft(x[2]+x[14], 13)
		x[10] ^= rotateLeft(x[6]+x[2], 18)
		x[3] ^= rotateLeft(x[15]+x[11], 7)
		x[7] ^= rotateLeft(x[3]+x[15], 9)
		x[11] ^= rotateLeft(x[7]+x[3], 13)
		x[15] ^= rotateLeft(x[11]+x[7], 18)
		// Row round.
		x[1] ^= rotateLeft(x[0]+x[3], 7)
		x[2] ^= rotateLeft(x[1]+x[0], 9)
		x[3] ^= rotateLeft(x[2]+x[1], 13)
		x[0] ^= rotateLeft(x[3]+x[2], 18)
		x[6] ^= rotateLeft(x[5]+x[4], 7)
		x[7] ^= rotateLeft(x[6]+x[5], 9)
		x[4] ^= rotateLeft(x[7]+x[6], 13)
		x[5] ^= rotateLeft(x[4]+x[7], 18)
		x[11] ^= rotateLeft(x[10]+x[9], 7)
		x[8] ^= rotateLeft(x[11]+x[10], 9)
		x[9] ^= rotateLeft(x[8]+x[11], 13)
		x[10] ^= rotateLeft(x[9]+x[8], 18)
		x[12] ^= rotateLeft(x[15]+x[14], 7)
		x[13] ^= rotateLeft(x[12]+x[15], 9)
		x[14] ^= rotateLeft(x[13]+x[12], 13)
		x[15] ^= rotateLeft(x[14]+x[13], 18)
	}
	for i := range x {
		x[i] += initial[i]
	}
}

func rotateLeft(value uint32, bits uint) uint32 { return value<<bits | value>>(32-bits) }
