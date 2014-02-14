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

//TODO may need lock since its changing v.viewRef
func ViewToViewRef(v *View) *ViewRef {
	var updates ByUpdate
	updates = v.getEntries()
	sort.Sort(updates)

	buf := new(bytes.Buffer)
	encoder := gob.NewEncoder(buf)
	encoder.Encode(updates)

	v.Ref = &ViewRef{sha1.Sum(buf.Bytes())}
	return v.Ref
}

// viewToViewRef computes view's ViewRef. Caller must lock mutex before calling viewToViewRef.
func viewToViewRef(view *View) *ViewRef {
	var updates ByUpdate
	updates = view.getEntries()
	sort.Sort(updates)

	buf := new(bytes.Buffer)
	encoder := gob.NewEncoder(buf)
	encoder.Encode(updates)

	return &ViewRef{sha1.Sum(buf.Bytes())}
}

type ByUpdate []Update

func (s ByUpdate) Len() int           { return len(s) }
func (s ByUpdate) Less(i, j int) bool { return s[i].Less(s[j]) }
func (s ByUpdate) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
