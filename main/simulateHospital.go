package main

import (
	"fmt"
	"log"
	sim "github.com/miselico/discrete-time-simulator/simulator"
	"strconv"
	//"math/rand"
)

type IntakeProcess struct{}

func (p IntakeProcess) Run(manager sim.ProcessManager) {
	corridor := manager.GetQueue("Corridor")
	for i := 0; i < 100; i++ {
		manager.Perform("Wait for next customer", 5)
		log.Println("Customer is there!, putting in corridor")
		err := corridor.Put(0, "Customer"+strconv.Itoa(i))
		if err != nil {
			log.Println("No space in the corridor, patient thrown on the street!")
		}
	}
	//manager.EndSimulation()
}

func (p IntakeProcess) GetName() string {
	return fmt.Sprintf("Intaker()")
}

type Operate struct{ period int64 }

func (o Operate) Run(manager sim.ProcessManager) {
	corridor := manager.GetQueue("Corridor")
	for i := 0; i < 200; i++ {
		manager.Perform("Drinking coffee", 3)
		patient, err := corridor.Get(2)
		if err != nil {
			log.Println("No work, going back for coffee")
			continue
		}
		//working very fast
		manager.Perform("Operating a patient" + patient.(string), 2)
	}

}

func (Operate) GetName() string {
	return "Operate"
}

func main() {
	
	queues := []sim.QueueDescription{sim.QueueDescription{"Corridor", 5}}
	proc := []sim.Process{IntakeProcess{}, Operate{}}
	sim.StartSimulation(queues, proc)
}
