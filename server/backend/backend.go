package backend

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
	"time"

	"database/sql"
	_ "github.com/mattn/go-sqlite3"

	"mateusbraga/gotf/view"
)

var (
	listener    net.Listener
	thisProcess view.Process
	db          *sql.DB
)

func Run(port uint) {
	var err error

	listener, err = net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Listening on address:", listener.Addr())

	thisProcess = view.Process{listener.Addr().String()}
	if err := view.PublishAddr(thisProcess.Addr); err != nil {
		log.Fatal(err)
	}

	dbName := fmt.Sprintf("./gotf.%v.db", port)
	os.Remove(dbName)
	db, err = sql.Open("sqlite3", dbName)
	if err != nil {
		log.Panic(err)
	}
	//TODO find place to close db

	sqls := []string{
		"create table prepare_request (consensus_id integer, highest_proposal_number integer)",
		"create table accepted_proposal (consensus_id integer, accepted_proposal integer)",
		"create table proposal_number (consensus_id integer, last_proposal_number integer)",
	}
	for _, sql := range sqls {
		_, err = db.Exec(sql)
		if err != nil {
			log.Panicf("%q: %s\n", err, sql)
		}
	}

	rpc.Accept(listener)
	// Sleep a little to give time to finish pending gorotines (from rpc)
	time.Sleep(3 * time.Second)
}
