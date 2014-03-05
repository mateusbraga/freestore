package view

import (
	"bytes"
	"crypto/sha1"
	"encoding/gob"
	"sort"
)

type ViewRef struct {
	Digest [sha1.Size]byte
}

func ViewToViewRef(v *View) ViewRef {
	var updates byUpdate
	updates = v.GetUpdates()

	// sort it to make it a canonical view
	sort.Sort(updates)

	buf := new(bytes.Buffer)
	encoder := gob.NewEncoder(buf)
	encoder.Encode(updates)

	return ViewRef{sha1.Sum(buf.Bytes())}
}

type byUpdate []Update

func (s byUpdate) Len() int           { return len(s) }
func (s byUpdate) Less(i, j int) bool { return s[i].Less(s[j]) }
func (s byUpdate) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
