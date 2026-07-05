// Package diff produces a unified-format diff between two strings.
// Uses Myers' O(ND) diff algorithm for quality output without external deps.
package diff

import (
	"fmt"
	"strings"
)

// Unified returns a unified diff between old and new content.
// Returns an empty string if there are no differences.
func Unified(oldContent, newContent string) string {
	if oldContent == newContent {
		return ""
	}

	oldLines := splitLines(oldContent)
	newLines := splitLines(newContent)

	edits := myers(oldLines, newLines)

	var sb strings.Builder
	sb.WriteString("--- a/PKGBUILD\n")
	sb.WriteString("+++ b/PKGBUILD\n")

	writeHunks(&sb, oldLines, newLines, edits)
	return sb.String()
}

// HasChanges returns true if the two content strings differ.
func HasChanges(oldContent, newContent string) bool {
	return oldContent != newContent
}

// edit kinds
const (
	opEq  = 0
	opDel = 1
	opIns = 2
)

type edit struct {
	op      int
	oldLine int // 0-indexed
	newLine int
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	// trailing newline creates an empty last element — keep it consistent
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// myers runs Myers' diff algorithm and returns a sequence of edits.
func myers(a, b []string) []edit {
	n, m := len(a), len(b)
	max := n + m
	if max == 0 {
		return nil
	}

	v := make([]int, 2*max+1)
	trace := [][]int{}

	for d := 0; d <= max; d++ {
		snap := make([]int, len(v))
		copy(snap, v)
		trace = append(trace, snap)

		for k := -d; k <= d; k += 2 {
			ki := k + max
			var x int
			if k == -d || (k != d && v[ki-1] < v[ki+1]) {
				x = v[ki+1]
			} else {
				x = v[ki-1] + 1
			}
			y := x - k
			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}
			v[ki] = x
			if x >= n && y >= m {
				return backtrack(trace, a, b, d, max)
			}
		}
	}
	return nil
}

func backtrack(trace [][]int, a, b []string, d, max int) []edit {
	x, y := len(a), len(b)
	edits := []edit{}

	for dd := d; dd > 0; dd-- {
		v := trace[dd]
		k := x - y
		ki := k + max

		var prevK int
		if k == -dd || (k != dd && v[ki-1] < v[ki+1]) {
			prevK = k + 1
		} else {
			prevK = k - 1
		}
		prevX := v[prevK+max]
		prevY := prevX - prevK

		for x > prevX && y > prevY {
			edits = append(edits, edit{opEq, x - 1, y - 1})
			x--
			y--
		}
		if dd > 0 {
			if x == prevX {
				edits = append(edits, edit{opIns, prevX, prevY})
			} else {
				edits = append(edits, edit{opDel, prevX, prevY})
			}
		}
		x, y = prevX, prevY
	}
	for x > 0 && y > 0 {
		edits = append(edits, edit{opEq, x - 1, y - 1})
		x--
		y--
	}

	// reverse
	for i, j := 0, len(edits)-1; i < j; i, j = i+1, j-1 {
		edits[i], edits[j] = edits[j], edits[i]
	}
	return edits
}

const context = 3

type hunk struct {
	start, end int
}

func writeHunks(sb *strings.Builder, a, b []string, edits []edit) {
	if len(edits) == 0 {
		return
	}

	var hunks []hunk
	i := 0
	for i < len(edits) {
		if edits[i].op == opEq {
			i++
			continue
		}
		start := i
		for i < len(edits) && edits[i].op != opEq {
			i++
		}
		hunks = append(hunks, hunk{start, i})
	}

	merged := mergeHunks(hunks, edits, context)
	for _, h := range merged {
		emitHunk(sb, a, b, edits, h[0], h[1])
	}
}

func mergeHunks(hunks []hunk, edits []edit, ctx int) [][2]int {
	if len(hunks) == 0 {
		return nil
	}
	var result [][2]int
	cur := [2]int{hunks[0].start, hunks[0].end}
	for _, h := range hunks[1:] {
		gap := 0
		for i := cur[1]; i < h.start; i++ {
			if edits[i].op == opEq {
				gap++
			}
		}
		if gap <= 2*ctx {
			cur[1] = h.end
		} else {
			result = append(result, cur)
			cur = [2]int{h.start, h.end}
		}
	}
	result = append(result, cur)
	return result
}

func emitHunk(sb *strings.Builder, a, b []string, edits []edit, start, end int) {
	// expand by context lines
	lo := start
	for lo > 0 && edits[lo-1].op == opEq {
		// count context lines before
		n := 0
		for j := lo - 1; j >= 0 && edits[j].op == opEq; j-- {
			n++
		}
		if n > context {
			break
		}
		lo--
	}
	hi := end
	n := 0
	for hi < len(edits) && edits[hi].op == opEq {
		hi++
		n++
		if n >= context {
			break
		}
	}

	// compute hunk header counts
	oldStart := edits[lo].oldLine + 1
	newStart := edits[lo].newLine + 1
	oldCount, newCount := 0, 0
	for _, e := range edits[lo:hi] {
		if e.op == opEq || e.op == opDel {
			oldCount++
		}
		if e.op == opEq || e.op == opIns {
			newCount++
		}
	}

	fmt.Fprintf(sb, "@@ -%d,%d +%d,%d @@\n", oldStart, oldCount, newStart, newCount)
	for _, e := range edits[lo:hi] {
		switch e.op {
		case opEq:
			sb.WriteString(" " + a[e.oldLine] + "\n")
		case opDel:
			sb.WriteString("-" + a[e.oldLine] + "\n")
		case opIns:
			sb.WriteString("+" + b[e.newLine] + "\n")
		}
	}
}
