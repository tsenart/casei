//go:build amd64 && !goexperiment.simd

package casei

// rootSkip32 and rootSkip64 scan complete vector blocks and return the length
// of the root-to-root prefix. Their callers check x/sys/cpu's GODEBUG-aware
// feature flags before entering the corresponding instruction set.
func rootSkip32(ptr *byte, n int, target, fold uint64) int
func rootSkip64(ptr *byte, n int, target, fold uint64) int
func literalSkip32(ptr *byte, n int, target, fold uint64) int
func literalSkip64(ptr *byte, n int, target, fold uint64) int
func runSkip32(ptr *byte, n int, target, fold uint64) int
func runSkip64(ptr *byte, n int, target, fold uint64) int
func runMask32(ptr *byte, target, fold uint64) uint32
func runMask64(ptr *byte, target, fold uint64) uint64
func probeSkip32(ptr *byte, n int, probe *asciiProbe) int
func probeSkip64(ptr *byte, n int, probe *asciiProbe) int
func probeVBMISkip64(ptr *byte, n int, probe *asciiVBMIProbe) int
func asciiOnlyProbeSkip64(ptr *byte, n int, probe *asciiProbe) int
func asciiPairDirectSkip64(ptr *byte, n int, probe *asciiPairProbe) int
func asciiPairDirectVBMISkip64(ptr *byte, n int, probe *asciiPairVBMIProbe) int
func asciiPairShortSkip64(ptr *byte, n int, probe *asciiPairProbe) int
func pairSetSkip32(ptr *byte, n int, filter *rootFilter) int
func pairSetSkip64(ptr *byte, n int, filter *rootFilter) int
func pairShuftiSkip64(ptr *byte, n int, filter *pairShuftiFilter) int
func pairShuftiWithOnesSkip64(ptr *byte, n int, filter *pairShuftiFilter) int
func pairPairSkip64(ptr *byte, n int, filter *pairPairFilter) int
func pairPairVBMISkip64(ptr *byte, n int, filter *pairPairVBMIFilter) int
func pairPairWordSkip64(ptr *byte, n int, filter *pairPairFilter) int
func pairSecondSkip32(ptr *byte, n int, filter *rootFilter) int
func pairSecondSkip64(ptr *byte, n int, filter *rootFilter) int
func pairSkip32(ptr *byte, n int, first, firstFold, second, secondFold uint64) int
func pairSkip64(ptr *byte, n int, first, firstFold, second, secondFold uint64) int
func filterSkip32(ptr *byte, n int, filter *rootFilter) int
func filterSkip64(ptr *byte, n int, filter *rootFilter) int
func tripleSkip32(ptr *byte, n int, filter *tripleFilter) int
func tripleSkip64(ptr *byte, n int, filter *tripleFilter) int
func tripleShuftiSkip64(ptr *byte, n int, filter *tripleShuftiFilter) int
func asciiPairAnchorSkip64(ptr *byte, n int, filter *asciiPairAnchorFilter) int
func asciiPairAnchorVBMISkip64(ptr *byte, n int, filter *asciiPairVBMIAnchorFilter) int
func tripleSharedPrefixSkip64(ptr *byte, n int, filter *tripleFilter) int
func tripleASCIIUTF8Skip64(ptr *byte, n int, filter *tripleFilter) int
func tripleMixedSkip64(ptr *byte, n int, filter *tripleFilter) int
func triplePairSkip32(ptr *byte, n int, filter *tripleFilter) int
func triplePairSkip64(ptr *byte, n int, filter *tripleFilter) int
