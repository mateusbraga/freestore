package view

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"log"
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
	for _, loopUpdate := range updates {
		fmt.Fprintf(buf, "%v", loopUpdate)
	}

	log.Println(buf.Bytes())

	return ViewRef{sha1.Sum(buf.Bytes())}
}

type byUpdate []Update

func (s byUpdate) Len() int           { return len(s) }
func (s byUpdate) Less(i, j int) bool { return s[i].Less(s[j]) }
func (s byUpdate) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
