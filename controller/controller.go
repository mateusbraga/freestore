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
	CurrentView   *view.View
}

type ProcessStatus struct {
	Process view.Process
	Running bool
}

func startProcess(process view.Process) {
	_, port, err := net.SplitHostPort(process.Addr)
	if err != nil {
		log.Fatal(err)
	}
	server := exec.Command("server", "-p", port, "-join=false")
	if err := server.Start(); err != nil {
		log.Fatal(err)
	}
}

func processHandler(w http.ResponseWriter, r *http.Request) {
	if ok, err := path.Match("/process/all/start", r.URL.Path); err == nil && ok {
		for _, status := range status.ProcessStatus {
			if !status.Running {
				startProcess(status.Process)
			}
		}
	} else if ok, err := path.Match("/process/all/terminate", r.URL.Path); err == nil && ok {
		for _, status := range status.ProcessStatus {
			if status.Running {
				sendTerminateProcess(status.Process)
			}
		}
	} else if ok, err := path.Match("/process/*/start", r.URL.Path); err == nil && ok {
		process, _ := path.Split(r.URL.Path[9:])
		process = process[:len(process)-1]
		startProcess(view.Process{process})
	} else if ok, err := path.Match("/process/*/terminate", r.URL.Path); err == nil && ok {
		process, _ := path.Split(r.URL.Path[9:])
		process = process[:len(process)-1]
		sendTerminateProcess(view.Process{process})
	} else if ok, err := path.Match("/process/*/join", r.URL.Path); err == nil && ok {
		process, _ := path.Split(r.URL.Path[9:])
		process = process[:len(process)-1]
		sendJoinProcess(view.Process{process})
	} else if ok, err := path.Match("/process/*/leave", r.URL.Path); err == nil && ok {
		process, _ := path.Split(r.URL.Path[9:])
		process = process[:len(process)-1]
		sendLeaveProcess(view.Process{process})
	} else if ok, err := path.Match("/process/*/view", r.URL.Path); err == nil && ok {
		process, _ := path.Split(r.URL.Path[9:])
		process = process[:len(process)-1]
		sendGetCurrentView(view.Process{process})
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func list(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles("page/list.html")
	if err != nil {
		log.Fatal(err)
	}

	t.Execute(w, status)
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
	if sendPingProcess(status.Process) {
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
	http.HandleFunc("/process/", processHandler)
	http.HandleFunc("/doozerd/", doozerdHandler)

	http.Handle("/css/", http.StripPrefix("/css/", http.FileServer(http.Dir(filepath.Join(cwd, "page/css")))))
	http.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir(filepath.Join(cwd, "page/js")))))

	go failureDetector()

	log.Println("Listening at http://localhost:9182")

	log.Fatal(http.ListenAndServe(":9182", nil))
}

func init() {
	//addr, err := view.GetRunningServer()
	//if err != nil {
	//log.Fatal(err)
	//}

	//sendGetCurrentView(view.Process{addr})

	// Init current view
	currentView = view.New()
	currentView.AddUpdate(view.Update{view.Join, view.Process{"[::]:5000"}})
	currentView.AddUpdate(view.Update{view.Join, view.Process{"[::]:5001"}})
	currentView.AddUpdate(view.Update{view.Join, view.Process{"[::]:5002"}})

	v := view.New()
	status.CurrentView = &v
	status.CurrentView.Set(&currentView)
	// Init status
	for i := 5000; i < 5020; i++ {
		status.ProcessStatus = append(status.ProcessStatus, &ProcessStatus{view.Process{fmt.Sprintf("[::]:%v", i)}, false})
	}
	checkDoozerdStatus()
}

// -------- Doozerd ------------------

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

func checkDoozerdStatus() {
	server := exec.Command("pgrep", "doozerd")
	if err := server.Run(); err != nil {
		status.Doozerd = false
	} else {
		status.Doozerd = true
	}
}

func doozerdHandler(w http.ResponseWriter, r *http.Request) {
	if ok, err := path.Match("/doozerd/start", r.URL.Path); err == nil && ok { // Terminate all
		startDoozerd()
	} else if ok, err := path.Match("/doozerd/kill", r.URL.Path); err == nil && ok {
		stopDoozerd()
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

// --------- Send Requests ---------------
func sendJoinProcess(process view.Process) {
	client, err := rpc.Dial("tcp", process.Addr)
	if err != nil {
		log.Println(err)
		return
	}
	defer client.Close()

	var value bool
	err = client.Call("ControllerRequest.Join", currentView, &value)
	if err != nil {
		log.Println(err)
		return
	}
}

func sendLeaveProcess(process view.Process) {
	client, err := rpc.Dial("tcp", process.Addr)
	if err != nil {
		log.Println(err)
		return
	}
	defer client.Close()

	var value bool
	err = client.Call("ControllerRequest.Leave", false, &value)
	if err != nil {
		log.Println(err)
		return
	}
}

func sendTerminateProcess(process view.Process) {
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

func sendPingProcess(process view.Process) bool {
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

// sendGetCurrentView asks process for the currentView
func sendGetCurrentView(process view.Process) {
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

	currentView.Set(&newView)
	status.CurrentView.Set(&newView)
	fmt.Println("New Current View:", currentView)
}
