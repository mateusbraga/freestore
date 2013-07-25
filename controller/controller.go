/*
This is a controller to monitor servers
*/
package main

import (
	"fmt"
	"html/template"
	"log"
	"mateusbraga/gotf/view"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"time"
)

var (
	currentView view.View
	status      Status = Status{}
)

type Status struct {
	Doozerd       bool
	ProcessStatus []*ProcessStatus
}

type ProcessStatus struct {
	Process view.Process
	Running bool
}

func testConsensusProcess(process view.Process) {
	client, err := rpc.Dial("tcp", process.Addr)
	if err != nil {
		log.Println(err)
		return
	}
	defer client.Close()

	var value int

	err = client.Call("ControllerRequest.Consensus", 6, &value)
	if err != nil {
		log.Println(err)
		return
	}
}

func terminateProcess(process view.Process) {
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

func pingProcess(process view.Process) bool {
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

func startProcess(process view.Process) {
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
	if !status.Doozerd {
		startDoozerd()
	}

	if ok, err := path.Match("/start/", r.URL.Path); err == nil && ok {
		for _, status := range status.ProcessStatus {
			if !status.Running {
				startProcess(status.Process)
			}
		}
	} else if ok, err := path.Match("/start/*", r.URL.Path); err == nil && ok {
		startProcess(view.Process{r.URL.Path[7:]})
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func testConsensus(w http.ResponseWriter, r *http.Request) {
	if ok, err := path.Match("/testConsensus/", r.URL.Path); err == nil && !ok { // Terminate all
		testConsensusProcess(view.Process{r.URL.Path[15:]})
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func terminate(w http.ResponseWriter, r *http.Request) {
	if ok, err := path.Match("/terminate/", r.URL.Path); err == nil && ok { // Terminate all
		for _, status := range status.ProcessStatus {
			if status.Running {
				terminateProcess(status.Process)
			}
		}

		stopDoozerd()
	} else if ok, err := path.Match("/terminate/*", r.URL.Path); err == nil && ok {
		terminateProcess(view.Process{r.URL.Path[11:]})
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func startDoozerd() {
	server := exec.Command("doozerd")
	if err := server.Start(); err != nil {
		log.Fatal(err)
	}

	status.Doozerd = true
}

func stopDoozerd() {
	server := exec.Command("pkill", "-9", "doozerd")
	if err := server.Start(); err != nil {
		log.Fatal(err)
	}

	status.Doozerd = false
}

func startDoozerdHandler(w http.ResponseWriter, r *http.Request) {
	startDoozerd()
	http.Redirect(w, r, "/", http.StatusFound)
}

func terminateDoozerdHandler(w http.ResponseWriter, r *http.Request) {
	stopDoozerd()
	http.Redirect(w, r, "/", http.StatusFound)
}

func list(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles("page/list.html")
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
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/", list)
	http.HandleFunc("/terminate/", terminate)
	http.HandleFunc("/start/", start)
	http.HandleFunc("/startDoozerd/", startDoozerdHandler)
	http.HandleFunc("/terminateDoozerd/", terminateDoozerdHandler)
	http.Handle("/css/", http.StripPrefix("/css/", http.FileServer(http.Dir(filepath.Join(cwd, "page/css")))))
	http.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir(filepath.Join(cwd, "page/js")))))
	http.HandleFunc("/testConsensus/", testConsensus)

	go failureDetector()

	log.Println("Listening at http://localhost:9182")

	log.Fatal(http.ListenAndServe(":9182", nil))
}

func init() {
	//addr, err := view.GetRunningServer()
	//if err != nil {
	//log.Fatal(err)
	//}

	//GetCurrentView(view.Process{addr})

	// Init current view
	currentView = view.New()
	currentView.AddUpdate(view.Update{view.Join, view.Process{":5000"}})
	currentView.AddUpdate(view.Update{view.Join, view.Process{":5001"}})
	currentView.AddUpdate(view.Update{view.Join, view.Process{":5002"}})

	// Init status
	for _, proc := range currentView.GetMembers() {
		status.ProcessStatus = append(status.ProcessStatus, &ProcessStatus{proc, false})
	}
	checkDoozerdStatus()
}

// GetCurrentViewClient asks process for the currentView
func GetCurrentView(process view.Process) {
	client, err := rpc.Dial("tcp", process.Addr)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	var newView view.View
	client.Call("ClientRequest.GetCurrentView", 0, &newView)
	if err != nil {
		log.Fatal(err)
	}

	currentView.Set(newView)
	fmt.Println("New Current View:", currentView)
}
