package ethutil

import (
	"math/big"
)

var BigInt0 *big.Int = big.NewInt(0)

// True
var BigTrue *big.Int = big.NewInt(1)

// False
var BigFalse *big.Int = big.NewInt(0)

// Returns the power of two integers
func BigPow(a, b int) *big.Int {
	c := new(big.Int)
	c.Exp(big.NewInt(int64(a)), big.NewInt(int64(b)), big.NewInt(0))

	return c
}

// Like big.NewInt(uint64); this takes a string instead.
func Big(num string) *big.Int {
	n := new(big.Int)
	n.SetString(num, 0)

	return n
}

// Like big.NewInt(uint64); this takes a byte buffer instead.
func BigD(data []byte) *big.Int {
	n := new(big.Int)
	n.SetBytes(data)

	return n
}

func BigToBytes(num *big.Int, base int) []byte {
	ret := make([]byte, base/8)

	return append(ret[:len(ret)-len(num.Bytes())], num.Bytes()...)
}
