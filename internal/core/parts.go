// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"regexp"
	"strconv"
)

// Parts is a split Takeout measured against the number of files the person
// said Google gave them.
//
// Nothing inside an archive says "part 2 of 3": Google shows the count only to
// the person, on the Takeout page. So the count is theirs to declare, and the
// import waits until that many archives are here. The numbers Google puts in
// the names (takeout-…-001.zip, -002, …) only sharpen what the page can say:
// which part is missing, and a declared count that is lower than a part
// already received. They are never required, because a name Google or the
// browser changed must not block an export that is otherwise whole.
type Parts struct {
	Expected int // what the person declared; 0 until they do
	Have     int // archives on disk
	// Missing lists the part numbers in 1..Expected not seen yet. It is filled
	// only when every name carried a number, so a renamed file cannot make a
	// part look missing.
	Missing []int
	// TooMany means more archives are here than declared, or a part numbered
	// above the declared count: the number is wrong, and importing now would
	// import an export the person thinks is something else.
	TooMany bool
}

// Complete reports whether every declared part is here and nothing points to
// the count being wrong.
func (p Parts) Complete() bool {
	return p.Expected > 0 && !p.TooMany && p.Have == p.Expected && len(p.Missing) == 0
}

// partNumber is the number Google appends to each archive. The match is
// anchored on the end, so a prefix added to tell apart two files with the same
// name does not hide it.
var partNumber = regexp.MustCompile(`(?i)-(\d{1,4})\.zip$`)

// CountParts measures the archive names on disk against the declared count.
func CountParts(names []string, expected int) Parts {
	p := Parts{Expected: expected, Have: len(names)}
	if expected <= 0 {
		return p
	}

	seen := map[int]bool{}
	numbered := true // no names yet counts as numbered: every part is missing
	for _, name := range names {
		m := partNumber.FindStringSubmatch(name)
		if m == nil {
			numbered = false
			continue
		}
		n, _ := strconv.Atoi(m[1])
		if n > expected {
			p.TooMany = true
		}
		seen[n] = true
	}

	if numbered {
		// Count distinct part NUMBERS, not files. The same part re-sent under a
		// hash prefix (upload.go) still ends in -NNN.zip, so two files can be the
		// same part; counting files would let a duplicate stand in for a part
		// that never arrived and start the import on an incomplete Takeout.
		p.Have = len(seen)
		for n := 1; n <= expected; n++ {
			if !seen[n] {
				p.Missing = append(p.Missing, n)
			}
		}
	}
	if p.Have > expected {
		p.TooMany = true
	}
	return p
}
