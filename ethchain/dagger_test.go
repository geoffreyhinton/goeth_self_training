package ethchain

import (
	"math/big"
	"testing"

	"github.com/geoffreyhinton/goeth_self_training/ethutil"
)

func BenchmarkDaggerSearch(b *testing.B) {
	hash := big.NewInt(0)
	diff := ethutil.BigPow(2, 36)
	o := big.NewInt(0) // nonce doesn't matter. We're only testing against speed, not validity

	// Reset timer so the big generation isn't included in the benchmark
	b.ResetTimer()
	// Validate
	DaggerVerify(hash, diff, o)
}
