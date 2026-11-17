package cfgsync

// maxDiffCells caps the LCS table; larger middles are shown as one removed and one added block.
const maxDiffCells = 4_000_000

// DiffLine is one line of a diff: Op is '=', '-' (only in dst) or '+' (only in src).
type DiffLine struct {
	Op   rune
	Text string
}

// DiffLines lists the lines that turn dst into src.
func DiffLines(dst, src []string) []DiffLine {
	prefix := 0
	for prefix < len(dst) && prefix < len(src) && dst[prefix] == src[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(dst)-prefix && suffix < len(src)-prefix && dst[len(dst)-1-suffix] == src[len(src)-1-suffix] {
		suffix++
	}
	midDst, midSrc := dst[prefix:len(dst)-suffix], src[prefix:len(src)-suffix]

	var out []DiffLine
	for _, l := range dst[:prefix] {
		out = append(out, DiffLine{'=', l})
	}
	out = append(out, diffMiddle(midDst, midSrc)...)
	for _, l := range dst[len(dst)-suffix:] {
		out = append(out, DiffLine{'=', l})
	}
	return out
}

func diffMiddle(dst, src []string) []DiffLine {
	var out []DiffLine
	if len(dst)*len(src) > maxDiffCells {
		for _, l := range dst {
			out = append(out, DiffLine{'-', l})
		}
		for _, l := range src {
			out = append(out, DiffLine{'+', l})
		}
		return out
	}
	// lcs[i][j] is the common length of dst[i:] and src[j:].
	lcs := make([][]int32, len(dst)+1)
	for i := range lcs {
		lcs[i] = make([]int32, len(src)+1)
	}
	for i := len(dst) - 1; i >= 0; i-- {
		for j := len(src) - 1; j >= 0; j-- {
			if dst[i] == src[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	i, j := 0, 0
	for i < len(dst) && j < len(src) {
		switch {
		case dst[i] == src[j]:
			out = append(out, DiffLine{'=', dst[i]})
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			out = append(out, DiffLine{'-', dst[i]})
			i++
		default:
			out = append(out, DiffLine{'+', src[j]})
			j++
		}
	}
	for ; i < len(dst); i++ {
		out = append(out, DiffLine{'-', dst[i]})
	}
	for ; j < len(src); j++ {
		out = append(out, DiffLine{'+', src[j]})
	}
	return out
}
