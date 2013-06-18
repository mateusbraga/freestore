package main

type group struct {
    IsPartitionMode bool
} 

var myGroup group

func joinGroup() {}

func init() {
	//INIT CODE

	joinGroup()

}

func main() {

	handleStuff()
}

func handleStuff() {
	if myGroup.IsPartitionMode {
		// go for perfect Consistency and Availability
	} else {
		// find tradeoff for operation, user and data.
		// Should it run? You got availability and need to restore consistency
		// Should it refuse to run? You will keep consistent
	}
}
