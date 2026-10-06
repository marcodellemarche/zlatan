// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"reflect"
	"testing"
)

func TestCountParts(t *testing.T) {
	const a, b, c = "takeout-20261006T101500Z-001.zip", "takeout-20261006T101500Z-002.zip", "takeout-20261006T101500Z-003.zip"
	cases := []struct {
		name     string
		names    []string
		expected int
		complete bool
		missing  []int
		tooMany  bool
	}{
		{"nothing declared yet", []string{a}, 0, false, nil, false},
		{"one of one", []string{a}, 1, true, nil, false},
		{"all three, any order", []string{c, a, b}, 3, true, nil, false},
		{"says which part is missing", []string{a, c}, 3, false, []int{2}, false},
		{"nothing arrived yet", nil, 2, false, []int{1, 2}, false},
		// The person said 2, but part 3 is here: the count is wrong, and
		// importing 1 and 2 would leave part 3 out.
		{"a part above the count", []string{a, c}, 2, false, []int{2}, true},
		{"more files than declared", []string{a, b, c}, 2, false, nil, true},
		// A name without Google's number (renamed by the browser) never blocks:
		// the count alone decides, and no part is claimed missing on a guess.
		{"renamed file, count matches", []string{a, "takeout (1).zip"}, 2, true, nil, false},
		{"renamed file, count short", []string{"photos.zip"}, 2, false, nil, false},
		// A same-name upload of different content is stored under a hash
		// prefix; the number at the end still reads.
		{"hash prefix keeps the number", []string{"0123456789ab-" + a, b}, 2, true, nil, false},
		// A duplicate of part 1 (resent under a hash prefix) is still one part:
		// it must not stand in for a part that never arrived.
		{"a resent part 1 is still one part", []string{a, "0123456789ab-" + a}, 2, false, []int{2}, false},
		{"a resent only part is complete", []string{a, "0123456789ab-" + a}, 1, true, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := CountParts(tc.names, tc.expected)
			if p.Complete() != tc.complete {
				t.Errorf("Complete() = %v, want %v (%+v)", p.Complete(), tc.complete, p)
			}
			if !reflect.DeepEqual(p.Missing, tc.missing) {
				t.Errorf("Missing = %v, want %v", p.Missing, tc.missing)
			}
			if p.TooMany != tc.tooMany {
				t.Errorf("TooMany = %v, want %v", p.TooMany, tc.tooMany)
			}
		})
	}
}
