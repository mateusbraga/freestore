package view

import (
	//"log"
	"errors"
	"fmt"
	"github.com/ha/doozer"
	"net"
)

func PublishAddr(addr string) error {
	doozerConn, err := doozer.Dial("localhost:8046")
	if err != nil {
		return errors.New(fmt.Sprint("Failed to connect to Doozer: ", err))
	}
	defer doozerConn.Close()

	rev, err := doozerConn.Rev()
	if err != nil {
		return err
	}

	files, err := doozerConn.Getdir("/goft/servers/", rev, 0, -1)
	if err != nil && err.Error() != "NOENT" {
		return err
	}

	id := len(files)
	_, err = doozerConn.Set(fmt.Sprintf("/goft/servers/%d", id), 0, []byte(addr))
	for err != nil && err.Error() == "REV_MISMATCH" {
		id++
		_, err = doozerConn.Set(fmt.Sprintf("/goft/servers/%d", id), 0, []byte(addr))
	}
	if err != nil {
		return err
	}

	return nil
}

func GetRunningServer() (string, error) {
	doozerConn, err := doozer.Dial("localhost:8046")
	if err != nil {
		return "", err
	}
	defer doozerConn.Close()

	rev, err := doozerConn.Rev()
	if err != nil {
		return "", err
	}

	files, err := doozerConn.Getdir("/goft/servers", rev, 0, -1)
	if err != nil {
		return "", err
	}
	//log.Println(files)

	for _, file := range files {
		path := fmt.Sprintf("/goft/servers/%v", file)
		addr, rev, err := doozerConn.Get(path, nil)
		if err != nil {
			return "", err
		}

		//fmt.Println(string(addr))
		conn, err := net.Dial("tcp", string(addr))
		if err != nil {
			//fmt.Println("falhou dial", err)
			err := doozerConn.Del(path, rev)
			if err != nil {
				return "", err
			}
			continue
		}
		defer conn.Close()

		if err != nil {
			return "", err
		}
		//log.Println("Did it!")
		return conn.RemoteAddr().String(), nil
	}
	return "", errors.New("Failed to find a running server")
}
