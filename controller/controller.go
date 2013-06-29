/*
This is a controller to monitor servers
*/
package main

import (
	"fmt"
	"log"
	"net/http"
	//"html"
	"html/template"
	"mateusbraga/gotf"
	"net"
	"net/rpc"
	"os/exec"
	"path"
	"time"
)

var (
	currentView gotf.View
	status      Status = Status{}
)

type Status struct {
	Doozerd       bool
	ProcessStatus []*ProcessStatus
}

type ProcessStatus struct {
	Process gotf.Process
	Running bool
}

func terminateProcess(process gotf.Process) {
	client, err := rpc.Dial("tcp", process.Addr)
	if err != nil {
		log.Println(err)
		return
	}
	defer client.Close()

	var value bool

	err = client.Call("ControllerRequest.Terminate", true, &value)
	if err != nil {
		log.Println(err)
		return
	}
}

func pingProcess(process gotf.Process) bool {
	client, err := rpc.Dial("tcp", process.Addr)
	if err != nil {
		return false
	}
	defer client.Close()

	var value bool

	err = client.Call("ControllerRequest.Ping", true, &value)
	if err != nil {
		return false
	}
	return true
}

func startProcess(process gotf.Process) {
	_, port, err := net.SplitHostPort(process.Addr)
	if err != nil {
		log.Fatal(err)
	}
	server := exec.Command("server", "-p", port)
	if err := server.Start(); err != nil {
		log.Fatal(err)
	}
}

func checkDoozerdStatus() {
	server := exec.Command("pgrep", "doozerd")
	if err := server.Run(); err != nil {
		status.Doozerd = false
	} else {
		status.Doozerd = true
	}
}

func start(w http.ResponseWriter, r *http.Request) {
	if ok, err := path.Match("/start/", r.URL.Path); err == nil && ok {
		for _, status := range status.ProcessStatus {
			if !status.Running {
				startProcess(status.Process)
			}
		}
	} else if ok, err := path.Match("/start/*", r.URL.Path); err == nil && ok {
		startProcess(gotf.Process{r.URL.Path[7:]})
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func terminate(w http.ResponseWriter, r *http.Request) {
	if ok, err := path.Match("/terminate/", r.URL.Path); err == nil && ok {
		for _, status := range status.ProcessStatus {
			if status.Running {
				terminateProcess(status.Process)
			}
		}
	} else if ok, err := path.Match("/terminate/*", r.URL.Path); err == nil && ok {
		terminateProcess(gotf.Process{r.URL.Path[11:]})
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func startDoozerd(w http.ResponseWriter, r *http.Request) {
	server := exec.Command("doozerd")
	if err := server.Start(); err != nil {
		log.Fatal(err)
	}

	status.Doozerd = true

	http.Redirect(w, r, "/", http.StatusFound)
}

func terminateDoozerd(w http.ResponseWriter, r *http.Request) {
	server := exec.Command("pkill", "-9", "doozerd")
	if err := server.Start(); err != nil {
		log.Fatal(err)
	}

	status.Doozerd = false

	http.Redirect(w, r, "/", http.StatusFound)
}

func list(w http.ResponseWriter, r *http.Request) {

	t, err := template.ParseFiles("list.html")
	if err != nil {
		log.Fatal(err)
	}

	t.Execute(w, status)

	//fmt.Fprintf(w, "Processes running: %v", processes)
}

func failureDetector() {
	for {
		select {
		case <-time.Tick(time.Second):
			for _, status := range status.ProcessStatus {
				go checkStatus(status)
			}
		}
	}
}

func checkStatus(status *ProcessStatus) {
	if pingProcess(status.Process) {
		status.Running = true
	} else {
		status.Running = false
	}
}

func main() {
	http.HandleFunc("/", list)
	http.HandleFunc("/terminate/", terminate)
	http.HandleFunc("/start/", start)
	http.HandleFunc("/startDoozerd/", startDoozerd)
	http.HandleFunc("/terminateDoozerd/", terminateDoozerd)

	go failureDetector()

	log.Fatal(http.ListenAndServe(":9182", nil))
}

func init() {
	//addr, err := gotf.GetRunningServer()
	//if err != nil {
	//log.Fatal(err)
	//}

	//GetCurrentView(gotf.Process{addr})

	currentView = gotf.NewView()
	currentView.AddUpdate(gotf.Update{gotf.Join, gotf.Process{":5000"}})
	currentView.AddUpdate(gotf.Update{gotf.Join, gotf.Process{":5001"}})
	currentView.AddUpdate(gotf.Update{gotf.Join, gotf.Process{":5002"}})

	for _, proc := range currentView.GetMembers() {
		status.ProcessStatus = append(status.ProcessStatus, &ProcessStatus{proc, false})
	}

	checkDoozerdStatus()
}

// GetCurrentViewClient asks process for the currentView
func GetCurrentView(process gotf.Process) {
	client, err := rpc.Dial("tcp", process.Addr)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	var newView gotf.View
	client.Call("ClientRequest.GetCurrentView", 0, &newView)
	if err != nil {
		log.Fatal(err)
	}

	currentView.Set(newView)
	fmt.Println("New Current View:", currentView)
}
