package view

import (
	"bytes"
	"crypto/sha1"
	"encoding/gob"
	"sort"
)

type ViewRef struct {
	digest [sha1.Size]byte
}

func viewToViewRef(view *View) ViewRef {
	buf := new(bytes.Buffer)
	encoder := gob.NewEncoder(buf)

	var updates ByUpdate
	updates = view.GetEntries()
	sort.Sort(updates)
	encoder.Encode(updates)

	return ViewRef{sha1.Sum(buf.Bytes())}
}

type ByUpdate []Update

func (s ByUpdate) Len() int           { return len(s) }
func (s ByUpdate) Less(i, j int) bool { return s[i].Less(s[j]) }
func (s ByUpdate) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
