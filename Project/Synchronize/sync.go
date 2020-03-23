package sync

import (
	"fmt"
	"math/rand"
	"strconv"
	"time"

	. "../config"
	"../network/localip"
)

type SyncChns struct {
	SendChn          chan Message
	RecChn           chan Message
	Online           chan bool
	IAmMaster        chan bool
	UpdatedAllOrders chan [NumElevs][NumFloors][NumButtons]bool
}

func Sync(id string, syncCh SyncChns, esmChns EsmChns) {
	const numPeers = NumElevs - 1
	idDig, _ := strconv.Atoi(id)
	var (
		elev            Elevator
		allOrders       [NumElevs][NumFloors][NumButtons]bool
		onlineIPs       []string
		receivedReceipt []string
		currentMsgID    int
		numTimeouts     int
		MergedAllOrders [NumElevs][NumFloors][NumButtons]bool
	)
	go func() {
		for {
			select {
			case newElev := <-esmChns.Elev:
				elev = newElev
				MergedAllOrders[idDig] = newElev.Orders
			}
		}
	}()

	localIP, err := localip.LocalIP()
	if err != nil {
		fmt.Println(err)
		localIP = "DISCONNECTED"
	}

	msgTimer := time.NewTimer(5 * time.Second)
	msgTimer.Stop()

	go func() {
		currentMsgID = rand.Intn(256)
		msg := Message{elev, allOrders, currentMsgID, false, localIP, id}
		for {
			syncCh.SendChn <- msg
			msgTimer.Reset(800 * time.Millisecond)
			time.Sleep(1 * time.Second)
		}
	}()

	for {
		select {
		case incomming := <-syncCh.RecChn:
			recID := incomming.LocalID
			if id != recID { //Hvis det ikke er fra oss selv, BYTTES TIL IP VED KJØRING PÅ FORSKJELLIGE MASKINER
				if !contains(onlineIPs, recID) {
					// Dersom heisen enda ikke er registrert, sjekker vi om vi nå er online og sjekker om vi er master
					onlineIPs = append(onlineIPs, recID)
					if len(onlineIPs) == numPeers {
						syncCh.Online <- true
						idDig, _ := strconv.Atoi(id)
						for i := 0; i < numPeers; i++ {
							theID, _ := strconv.Atoi(onlineIPs[i])
							if idDig > theID {
								syncCh.IAmMaster <- false
								break
							}
							syncCh.IAmMaster <- true
						}
						/*
							Dette er ved diff på IP:
							localDig, _ := strconv.Atoi(localIP[len(localIP)-3:])
							for i := 0; i <= numPeers; i++ {
								theIP := onlineIPs[i]
								lastDig, _ := strconv.Atoi(theIP[len(theIP)-3:])
								if localDig < lastDig {
									iAmMaster = false
									break
								}
							}
						*/
					}
				}
				if !incomming.Receipt {
					// Hvis det ikke er en kvittering, skal vi svare med kvittering
					msg := Message{elev, allOrders, incomming.MsgId, true, localIP, id}
					//sender ut fem kvitteringer på femti millisekunder
					for i := 0; i < 5; i++ {
						syncCh.SendChn <- msg
						time.Sleep(10 * time.Millisecond)
					}
				} else { // Hvis det er en kvittering
					if incomming.MsgId == currentMsgID {
						if !contains(receivedReceipt, recID) {
							receivedReceipt = append(receivedReceipt, recID)
							if len(receivedReceipt) == numPeers {
								numTimeouts = 0
								msgTimer.Stop()
								receivedReceipt = receivedReceipt[:0]
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
			}
		}
	}
}

func OrdersDist(syncCh SyncChns) {
	var (
		online    bool //initiates to false
		iAmMaster bool = true
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
			case b := <-syncCh.IAmMaster:
				if b {
					iAmMaster = true
				} else {
					iAmMaster = false
				}
			}

		}
	}()
	for {
		if online {
			if !iAmMaster {

			}
			fmt.Println("Online")
			if iAmMaster {
				fmt.Println(".. and I am master")
			} else {
				fmt.Println(".. and I am backup")
			}
			time.Sleep(5 * time.Second)
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
