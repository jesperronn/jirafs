package merge

import "strings"

// hunk replaces base[s:e] with repl.
type hunk struct {
	s, e int
	repl []string
	side int // 0 local, 1 remote
}

// Lines three-way merges text by line against base. clean is false when both
// sides changed overlapping (or touching) regions of base differently; merged
// is then meaningless and callers keep the local text.
func Lines(base, local, remote string) (merged string, clean bool) {
	b := strings.Split(base, "\n")
	l := strings.Split(local, "\n")
	r := strings.Split(remote, "\n")

	all := mergeHunks(diffHunks(b, l, 0), diffHunks(b, r, 1))
	var out []string
	pos := 0
	for i := 0; i < len(all); {
		cs, ce := all[i].s, all[i].e
		j := i + 1
		for j < len(all) && all[j].s <= ce {
			if all[j].e > ce {
				ce = all[j].e
			}
			j++
		}
		out = append(out, b[pos:cs]...)
		cluster := all[i:j]
		lt, lok := applySide(b, cs, ce, cluster, 0)
		rt, rok := applySide(b, cs, ce, cluster, 1)
		switch {
		case lok && rok:
			if !equalLines(lt, rt) {
				return "", false
			}
			out = append(out, lt...)
		case lok:
			out = append(out, lt...)
		default:
			out = append(out, rt...)
		}
		pos = ce
		i = j
	}
	out = append(out, b[pos:]...)
	return strings.Join(out, "\n"), true
}

// applySide returns base[cs:ce] with the hunks of side applied; ok is false
// when side has no hunk in the cluster.
func applySide(base []string, cs, ce int, cluster []hunk, side int) ([]string, bool) {
	var out []string
	pos := cs
	ok := false
	for _, h := range cluster {
		if h.side != side {
			continue
		}
		ok = true
		out = append(out, base[pos:h.s]...)
		out = append(out, h.repl...)
		pos = h.e
	}
	if !ok {
		return nil, false
	}
	out = append(out, base[pos:ce]...)
	return out, true
}

func equalLines(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// mergeHunks interleaves two start-ordered hunk lists by start position.
func mergeHunks(a, b []hunk) []hunk {
	out := make([]hunk, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) || j < len(b) {
		if j >= len(b) || (i < len(a) && a[i].s <= b[j].s) {
			out = append(out, a[i])
			i++
		} else {
			out = append(out, b[j])
			j++
		}
	}
	return out
}

// diffHunks returns the minimal line edits turning base into other, using an
// LCS table over the region between the common prefix and suffix.
func diffHunks(base, other []string, side int) []hunk {
	pre := 0
	for pre < len(base) && pre < len(other) && base[pre] == other[pre] {
		pre++
	}
	suf := 0
	for suf < len(base)-pre && suf < len(other)-pre && base[len(base)-1-suf] == other[len(other)-1-suf] {
		suf++
	}
	a := base[pre : len(base)-suf]
	b := other[pre : len(other)-suf]
	if len(a) == 0 && len(b) == 0 {
		return nil
	}

	dp := make([][]int32, len(a)+1)
	for i := range dp {
		dp[i] = make([]int32, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}

	var hunks []hunk
	var cur *hunk
	flush := func() {
		if cur != nil {
			hunks = append(hunks, *cur)
			cur = nil
		}
	}
	i, j := 0, 0
	for i < len(a) || j < len(b) {
		if i < len(a) && j < len(b) && a[i] == b[j] {
			flush()
			i++
			j++
			continue
		}
		if cur == nil {
			cur = &hunk{s: pre + i, e: pre + i, side: side}
		}
		if j >= len(b) || (i < len(a) && dp[i+1][j] >= dp[i][j+1]) {
			i++
			cur.e = pre + i
		} else {
			cur.repl = append(cur.repl, b[j])
			j++
		}
	}
	flush()
	return hunks
}
