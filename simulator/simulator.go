//Package simulator implements a very simple discrite time simulator
package simulator

import (
	"container/list"
	"log"
	"fmt"
	"sync"
)

type Process interface {
	Run(manager ProcessManager)
	GetName() string
}

type Queue interface {
	Get(timeout int64) (interface{}, error)
	Put(timeout int64, val interface{}) error
}

type ProcessManager interface {
	Perform(action string, duration int64)
	StartProcess(process Process)
	GetQueue(name string) Queue
	//do we need this? most likely
	EndSimulation()
}

type simulator struct {
	//this list contains all things which are waiting for a certain time-point in increasing order
	eventQueue     *list.List
	eventQueueLock *sync.Mutex

	queues              map[string]chan interface{}
	currentAbsoluteTime int64
	//	endsimulation                                            *sync.WaitGroup
	newProcess, blockedProcess, unBlockedProcess, endProcess, endsimulation chan *processManager
}

type QueueDescription struct {
	Name string
	Size int
}

type SimulationResult struct {
	//let us see
}

func StartSimulation(queues []QueueDescription, initialProcesses []Process) SimulationResult {
	sim := simulator{eventQueue: list.New(),
		eventQueueLock:      &sync.Mutex{},
		queues:              make(map[string]chan interface{}),
		currentAbsoluteTime: 0,
		newProcess:          make(chan *processManager),
		blockedProcess:      make(chan *processManager),
		unBlockedProcess:    make(chan *processManager),
		endProcess:          make(chan *processManager),
		endsimulation:       make(chan *processManager),
	}
	for _, qd := range queues {
		sim.queues[qd.Name] = make(chan interface{}, qd.Size)
	}
	for _, proc := range initialProcesses {
		go sim.schedule(proc)
	}
	return sim.start()
}

func (sim *simulator) start() SimulationResult {

	activeProcesses := 0
	blockedProcess := 0
	for {
		select {
		case _ = <-sim.newProcess:
			//log.Println("process started")
			activeProcesses++
		case _ = <-sim.endProcess:
			activeProcesses--
		case _ = <-sim.blockedProcess:
			blockedProcess++
		case _ = <-sim.unBlockedProcess:
			blockedProcess--
		case _ = <-sim.endsimulation:
			log.Println("Simulation end")
			//TODO correct the return
			return SimulationResult{}
		}
		if blockedProcess == activeProcesses {
			log.Println(fmt.Sprintf("Simulation step %d" ,sim.currentAbsoluteTime))
			sim.timeStep()
		}
	}

}

func (sim *simulator) timeStep() {
	//step the simulator
	el := sim.eventQueue.Front()
	if el == nil {
		panic("front of eventQueue is nil, this should never happen, when doing a time-step all processes (>0) are blocked and blocked processes have an entry in the list")
	}
	sim.eventQueue.Remove(el)
	eqe := el.Value.(eventQueueElement)
	sim.currentAbsoluteTime = eqe.absoluteTime
	//TODO this could loop over channels ??
	eqe.wg.Done()
}

func (sim *simulator) schedule(p Process) {
	man := &processManager{sim, p}
	sim.newProcess <- man
	go func() {
		p.Run(man)
		sim.endProcess <- man
	}()
}

type eventQueueElement struct {
	//this seemed a nice idea : only one link in the linked list would be locked, insertions could happen concurrently. However, if an insertion happens while you are crawling over the list undefined things might happen,
	//-> we use a 'global' lock instead
	//putBeforeLock * sync.Mutex
	//we use a wg and not a condition because it might happen that the clients are calling Wait() after Broadcast. For wag calling Wait() after Done() does not harm
	//it might be that the implementation of the final scheduling actually prevents this from ever happening.
	wg           *sync.WaitGroup
	absoluteTime int64
}

func newEQE(absoluteTime int64) eventQueueElement {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	return eventQueueElement{wg, absoluteTime}
}

//This could be changed to use chan's instead.
func (sim *simulator) scheduleWakeUp(after int64) *sync.WaitGroup {
	//note: after can be =0, this is used for time-outs on gets and puts on queueus. It is now also fine to sleep for a duration of 0
	if after < 0 {
		panic("after argument to scheduleWakeUp cannot be < 0")
	}
	//While scheduling a wakeup, nothing can be removed from the list because scheduling only happens during when a cycle is active.
	q := sim.eventQueue

	absoluteTime := sim.currentAbsoluteTime + after
	sim.eventQueueLock.Lock()
	defer sim.eventQueueLock.Unlock()
	for e := q.Front(); e != nil; e = e.Next() {
		eqe := e.Value.(eventQueueElement)
		if eqe.absoluteTime == absoluteTime {
			//re-use the wg
			return eqe.wg
		} else if eqe.absoluteTime > absoluteTime {
			//the new wakeup must be placed before
			newEqe := newEQE(absoluteTime)
			q.InsertBefore(newEqe, e)
			return newEqe.wg
		}
	}
	//the element should be put in the back!
	//it might be that there is nothing in the list yet
	newEqe := newEQE(absoluteTime)
	q.PushBack(newEqe)
	return newEqe.wg
}

func (sim *simulator) scheduleWakeUpChan(after int64) chan struct{} {
	wg := sim.scheduleWakeUp(after)
	timeoutChan := make(chan struct{}, 1)
	go func() {
		wg.Wait()
		timeoutChan <- struct{}{}
	}()
	return timeoutChan
}

type processManager struct {
	sim *simulator
	p   Process
}

func (pm *processManager) Perform(action string, duration int64) {
	log.Printf("Process : '%s' performing '%s' for %d timesteps\n", pm.p.GetName(), action, duration)
	if duration > 0 {
		//schedule a wake-up on the simulator
		wg := pm.sim.scheduleWakeUp(duration)
		//inform that this process will block
		pm.sim.blockedProcess <- pm
		wg.Wait()
		pm.sim.unBlockedProcess <- pm
	}
}

func (pm *processManager) StartProcess(process Process) {
	pm.sim.schedule(process)
}

func (pm *processManager) GetQueue(name string) Queue {
	chanWrapper := queue{pm.sim.queues[name], pm}
	return chanWrapper
}

func (pm *processManager) EndSimulation() {
	pm.sim.endsimulation <- pm
}

//internal queue type, implements Queue, interacts with the simulator
type queue struct {
	ch chan interface{}
	pm *processManager
}

type TimeOutError struct{}

//func TimeOutError

func (TimeOutError) Error() string {
	return "The operation timed out"
}

func (q queue) Get(timeout int64) (interface{}, error) {

	timeoutChan := q.pm.sim.scheduleWakeUpChan(timeout + 1)
	q.pm.sim.blockedProcess <- q.pm
	defer func() { q.pm.sim.unBlockedProcess <- q.pm }()
	select {
	//there could be a very subtle error here : what if two processes are waiting for each other? both time out and the first one then writes to the queue, which the second one could have received ###
	//but : we cannot prevent deadlick, livelock or startvation here!
	case <-timeoutChan:
		return struct{}{}, TimeOutError{}
	case val := <-q.ch:
		return val, nil
	}
}

func (q queue) Put(timeout int64, val interface{}) error {
	timeoutChan := q.pm.sim.scheduleWakeUpChan(timeout + 1)
	q.pm.sim.blockedProcess <- q.pm
	defer func() { q.pm.sim.unBlockedProcess <- q.pm }()
	select {
	//there could be a very subtle error here : what if two processes are waiting for each other? both time out and the first one then writes to the queue, which the second one could have received ###
	//but : we cannot prevent deadlick, livelock or startvation here!
	case <-timeoutChan:
		return TimeOutError{}
	case q.ch <- val:
		return nil
	}
}
