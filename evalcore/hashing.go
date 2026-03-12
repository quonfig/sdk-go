package evalcore

import (
	"math"

	"github.com/spaolacci/murmur3"
)

// HashZeroToOne hashes the given string using Murmur3 and returns a float64 in [0, 1).
func HashZeroToOne(value string) (float64, bool) {
	h32 := murmur3.New32()
	_, err := h32.Write([]byte(value))
	if err != nil {
		return 0, false
	}
	return float64(h32.Sum32()) / float64(math.MaxUint32), true
}
