package main

import (
	"flag"

	. "../StateMachine"
	"../config"
	. "../driver-go/elevio"
	"../sync"

	"../network/bcast"
	//	"time"
	//"fmt"
)

func main() {
	const NumElevs = config.NumElevs
	const NumFloors = config.NumFloors
	const NumButtons = config.NumButtons
	esmChns := config.EsmChns{
		Elev:             make(chan config.Elevator),
		CurrentAllOrders: make(chan [NumElevs][NumFloors][NumButtons]bool),
		Buttons:          make(chan ButtonEvent),
		Floors:           make(chan int),
	}

	Init("localhost:12345", NumFloors)

	/////// DETTE ER FRA SYNC ////////////
	syncChns := config.SyncChns{
		SendChn:   make(chan config.Message),
		RecChn:    make(chan config.Message),
		Online:    make(chan bool),
		IAmMaster: make(chan bool),
	}

	var id string
	flag.StringVar(&id, "id", "", "id of this peer")
	flag.Parse()

	port := 16576

	go bcast.Transmitter(port, syncChns.SendChn)
	go bcast.Receiver(port, syncChns.RecChn)
	go sync.Sync(id, syncChns, esmChns)
	go sync.OrdersDist(syncChns)
	/////////////

	//go SyncTest(esmChns.CurrentAllOrders, esmChns.Elev)

	go PollButtons(esmChns.Buttons)
	go PollFloorSensor(esmChns.Floors)
	go RunElevator(esmChns)

	for {

	}
}
