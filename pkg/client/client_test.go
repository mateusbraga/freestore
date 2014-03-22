package client

import (
	"bytes"
	"crypto/rand"
	"encoding/gob"
	"io"
	"log"
	"testing"

	"github.com/mateusbraga/freestore/pkg/view"
)

func TestRegisterMsgGob(t *testing.T) {
	updates := []view.Update{view.Update{Type: view.Join, Process: view.Process{"10.1.1.2:5000"}},
		view.Update{Type: view.Join, Process: view.Process{"10.1.1.3:5000"}},
		view.Update{Type: view.Join, Process: view.Process{"10.1.1.4:5000"}},
	}

	v1 := view.NewWithUpdates(updates...)

	msg := RegisterMsg{}
	msg.Value = createFakeData(512)
	msg.Timestamp = 1
	msg.ViewRef = view.ViewToViewRef(v1)

	buf := new(bytes.Buffer)
	encoder := gob.NewEncoder(buf)
	err := encoder.Encode(msg)
	if err != nil {
		t.Errorf(err.Error())
	}

	var msg2 RegisterMsg
	buf2 := bytes.NewReader(buf.Bytes())
	decoder := gob.NewDecoder(buf2)
	err = decoder.Decode(&msg2)
	if err != nil {
		t.Errorf(err.Error())
	}
}

func BenchmarkRegisterMsgGobEncode(b *testing.B) {
	updates := []view.Update{view.Update{Type: view.Join, Process: view.Process{"10.1.1.2:5000"}},
		view.Update{Type: view.Join, Process: view.Process{"10.1.1.3:5000"}},
		view.Update{Type: view.Join, Process: view.Process{"10.1.1.4:5000"}},
	}
	v1 := view.NewWithUpdates(updates...)

	msg := RegisterMsg{}
	msg.Value = createFakeData(512)
	msg.Timestamp = 1
	msg.ViewRef = view.ViewToViewRef(v1)

	buf := new(bytes.Buffer)

	for i := 0; i < b.N; i++ {
		encoder := gob.NewEncoder(buf)
		err := encoder.Encode(msg)
		if err != nil {
			b.Errorf(err.Error())
		}
		buf.Reset()
	}
}

func BenchmarkRegisterMsgGobDecode(b *testing.B) {
	updates := []view.Update{view.Update{Type: view.Join, Process: view.Process{"10.1.1.2:5000"}},
		view.Update{Type: view.Join, Process: view.Process{"10.1.1.3:5000"}},
		view.Update{Type: view.Join, Process: view.Process{"10.1.1.4:5000"}},
	}
	v1 := view.NewWithUpdates(updates...)

	msg := RegisterMsg{}
	msg.Value = createFakeData(512)
	msg.Timestamp = 1
	msg.ViewRef = view.ViewToViewRef(v1)

	buf := new(bytes.Buffer)
	var msg2 RegisterMsg

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		buf.Reset()
		encoder := gob.NewEncoder(buf)
		err := encoder.Encode(msg)
		if err != nil {
			b.Errorf(err.Error())
		}
		buf2 := bytes.NewReader(buf.Bytes())
		decoder := gob.NewDecoder(buf2)
		b.StartTimer()

		err = decoder.Decode(&msg2)
		if err != nil {
			b.Errorf(err.Error())
		}
	}
}

func createFakeData(size int) []byte {
	data := make([]byte, size)

	n, err := io.ReadFull(rand.Reader, data)
	if n != len(data) || err != nil {
		log.Fatalln("error to generate data:", err)
	}
	return data
}
