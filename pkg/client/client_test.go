package client

import (
	"bytes"
	"crypto/rand"
	"encoding/gob"
	"io"
	"log"
	"testing"
	"time"

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
	msg.View = v1

	buf := new(bytes.Buffer)
	encoder := gob.NewEncoder(buf)
	startTime := time.Now()
	err := encoder.Encode(msg)
	if err != nil {
		t.Errorf(err.Error())
	}
	endTime := time.Now()

	t.Logf("RegisterMsg %v encoded is %v bytes long and took %v to encode\n", msg, len(buf.Bytes()), endTime.Sub(startTime))

	var msg2 RegisterMsg
	buf2 := bytes.NewReader(buf.Bytes())
	decoder := gob.NewDecoder(buf2)
	err = decoder.Decode(&msg2)
	if err != nil {
		t.Errorf(err.Error())
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
