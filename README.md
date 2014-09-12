discrete-time-simulator
=======================

This is a work in progress discrete time simulator written in Go.

:warning: **The code is not usable in its current state** :warning:


The following are general ideas for the design:

* There is a global clock, which increments with the time needed till the next event
* The active components in the system are processes
* Processes can communicate with each other trough named queues
    * a send and receive on a queue can have a time-out
    * a queueu has a maximum size
* Processes only perceive relative time (no knowledge about absolute time)
* Processes are run in separate go routines
* All actions preformed are logged

Processes can

* Post new events to the system
* Perform actions which take a reported amount of time 
  * The time for the action is what is reported, not how long the simulation took.
* Sleep

