package mch_test

import (
	"math/rand"
	"testing"

	"github.com/mnencia/mchfuse/mch"
)

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randomString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func eqSlices(a, b []byte) bool {
	if (a == nil) != (b == nil) {
		return false
	}

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

// Test FOPEN_KEEP_CACHE. This is a little subtle: the automatic cache
// invalidation triggers if mtime or file size is changed, so only
// change content but no metadata.
func TestCache(t *testing.T) {
	var cf mch.CacheFile
	cf.Reset()

	if cf.Length() != 0 || cf.Offset() != 0 {
		t.Errorf("Cache file has wrong starting state")
	}

	whole_data := []byte("asdfbsdf")
	chunk1 := whole_data[:4]
	chunk2 := whole_data[4:]

	cf.Add(chunk1, 1024)
	if cf.Length() != 4 {
		t.Errorf("Incorrect length after first Add call")
	}
	if cf.Offset() != 1024 {
		t.Errorf("Incorrect offset after first Add call")
	}
	if false == eqSlices(cf.Content(), chunk1) {
		t.Errorf("Incorrect cache content")
	}

	cf.Add(chunk2, 1028)
	if cf.Length() != 8 {
		t.Errorf("Incorrect length after second Add call")
	}
	if cf.Offset() != 1024 {
		t.Errorf("Incorrect offset after second Add call")
	}
	if false == eqSlices(cf.Content(), whole_data) {
		t.Errorf("Incorrect cache content")
	}

	if cf.Full() == true {
		t.Errorf("Cache shouldn't be full yet")
	}

	rand_str := randomString(mch.CACHE_SIZE_SOFT_CAP - len(whole_data) - 1)
	cf.Add([]byte(rand_str), 1)
	if cf.Full() == true {
		t.Errorf("Cache shouldn't be full yet")
	}

	cf.Add([]byte("a"), 1)
	if cf.Full() == false {
		t.Errorf("Cache should be full")
	}

	cf.Reset()
	if cf.Length() != 0 || cf.Offset() != 0 {
		t.Errorf("Cache file has wrong state after reset")
	}
}
