package sync

import (
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"../config"
	"../network/localip"
)

func Sync(id string, syncCh config.SyncChns, esmChns config.EsmChns) {
	const numPeers = config.NumElevs - 1
	idDig, _ := strconv.Atoi(id)
	idDig--
	masterID := idDig
	var (
		elev               config.Elevator
		onlineIPs          []string
		receivedReceipt    []string
		currentMsgID       int
		numTimeouts        int
		updatedLocalOrders [config.NumElevs][config.NumFloors][config.NumButtons]bool
		currentAllOrders   [config.NumElevs][config.NumFloors][config.NumButtons]bool
		online             bool
		allElevs           [config.NumElevs]config.Elevator
	)
	go func() {
		for {
			select {
			case b := <-syncCh.Online:
				if b {
					online = true
					fmt.Println("Yaho, we are online!")
				} else {
					online = false
					fmt.Println("Boo, we are offline.")

				}
			case elev := <-esmChns.Elev:
				if updatedLocalOrders[idDig] != elev.Orders {
					updatedLocalOrders[idDig] = elev.Orders
				}
				allElevs[idDig] = elev
			}
		}
	}()

	localIP, err := localip.LocalIP()
	if err != nil {
		fmt.Println(err)
		localIP = "DISCONNECTED"
	}

	go func() {
		for {
			if currentAllOrders != updatedLocalOrders {
				if !online {
					updatedLocalOrders = mergeAllOrders(idDig, updatedLocalOrders)
					esmChns.CurrentAllOrders <- updatedLocalOrders
					currentAllOrders = updatedLocalOrders
				}
			}
			time.Sleep(20 * time.Millisecond)
		}
	}()

	msgTimer := time.NewTimer(5 * time.Second)
	msgTimer.Stop()
	go func() {
		for {
			currentMsgID = rand.Intn(256)
			msg := config.Message{elev, updatedLocalOrders, currentMsgID, false, localIP, id}
			syncCh.SendChn <- msg
			msgTimer.Reset(800 * time.Millisecond)
			time.Sleep(1 * time.Second)
		}
	}()

	for {
		select {
		case incomming := <-syncCh.RecChn:
			recID := incomming.LocalID
			recIDDig, _ := strconv.Atoi(recID)
			recIDDig--
			if id != recID { //Hvis det ikke er fra oss selv, BYTTES TIL IP VED KJØRING PÅ FORSKJELLIGE MASKINER
				if !contains(onlineIPs, recID) {
					// Dersom heisen enda ikke er registrert, sjekker vi om vi nå er online og sjekker om vi er master
					onlineIPs = append(onlineIPs, recID)
					if len(onlineIPs) == numPeers {
						syncCh.Online <- true
						for i := 0; i < numPeers; i++ {
							theID, _ := strconv.Atoi(onlineIPs[i])
							theID--
							if theID < masterID {
								masterID = theID
							}
						}
					}
				}
				// OBS: CurrentAllOrders er alltid det samme som ligger på heisen. Når online er
				// dette alltid de ordrene som er bekreftet at de andre har mottat.
				if !incomming.Receipt {
					if online {
						allElevs[recIDDig] = incomming.Elev
						allElevs[recIDDig].Orders = incomming.AllOrders[recIDDig]
						if currentAllOrders != incomming.AllOrders {
							// Hvis vi mottar noe nytt
							if idDig == masterID {
								// Hvis jeg er master
								updatedLocalOrders = CostFunction(allElevs)
								// Lokale endringer tas med i elev uansett
							} else if recIDDig == masterID {
								// Hvis det er melding fra master
								if updatedLocalOrders != currentAllOrders {
									updatedLocalOrders = mergeLocalOrders(idDig, incomming.AllOrders, updatedLocalOrders)
									// Hvis det er lokale endringer som har skjedd som vi ikke har
									// fått bekreftelse på
								} else {
									// Hvis alle er up to date med mine lokale bestillinger
									updatedLocalOrders = incomming.AllOrders
								}
							}
						}
					}
					msg := config.Message{elev, updatedLocalOrders, incomming.MsgId, true, localIP, id}
					for i := 0; i < 5; i++ {
						syncCh.SendChn <- msg
						time.Sleep(10 * time.Millisecond)
					}
				} else {
					if incomming.MsgId == currentMsgID {
						if !contains(receivedReceipt, recID) {
							receivedReceipt = append(receivedReceipt, recID)
							if len(receivedReceipt) == numPeers {
								numTimeouts = 0
								msgTimer.Stop()
								receivedReceipt = receivedReceipt[:0]
								if currentAllOrders != updatedLocalOrders {
									esmChns.CurrentAllOrders <- updatedLocalOrders
									currentAllOrders = updatedLocalOrders
								}
							}
						}
					}
				}
			}
		case <-msgTimer.C:
			numTimeouts++
			if numTimeouts > 2 {
				syncCh.Online <- false
				fmt.Println("Three timeouts in a row")
				numTimeouts = 0
				onlineIPs = onlineIPs[:0]
				masterID = idDig
			}
		}
	}
}

func contains(elevs []string, str string) bool {
	for _, a := range elevs {
		if a == str {
			return true
		}
	}
	return false
}

func costfcn(id int, current [config.NumElevs][config.NumFloors][config.NumButtons]bool, new [config.NumFloors][config.NumButtons]bool) [config.NumElevs][config.NumFloors][config.NumButtons]bool {
	current[id] = new
	allOrderMat := mergeAllOrders(0, current)
	return allOrderMat
}

func mergeAllOrders(id int, all [config.NumElevs][config.NumFloors][config.NumButtons]bool) [config.NumElevs][config.NumFloors][config.NumButtons]bool {
	var merged [config.NumElevs][config.NumFloors][config.NumButtons]bool
	merged[id] = all[id]
	for elev := 0; elev < config.NumElevs; elev++ {
		if elev == id {
			continue
		}
		for floor := 0; floor < config.NumFloors; floor++ {
			for btn := 0; btn < config.NumButtons; btn++ {
				if all[elev][floor][btn] && btn != config.NumButtons-1 {
					merged[id][floor][btn] = true
					merged[elev][floor][btn] = false
				}
			}
		}
	}
	return merged
}

func mergeLocalOrders(id int, inc [config.NumElevs][config.NumFloors][config.NumButtons]bool, local [config.NumElevs][config.NumFloors][config.NumButtons]bool) [config.NumElevs][config.NumFloors][config.NumButtons]bool {
	var merged [config.NumElevs][config.NumFloors][config.NumButtons]bool
	merged = inc
	for floor := 0; floor < config.NumFloors; floor++ {
		for btn := 0; btn < config.NumButtons; btn++ {
			if local[id][floor][btn] {
				merged[id][floor][btn] = true
			}
		}
	}
	return merged
}
