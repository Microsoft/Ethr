//-----------------------------------------------------------------------------
// Copyright (C) Microsoft. All rights reserved.
// Licensed under the MIT license.
// See LICENSE.txt file in the project root for full license information.
//-----------------------------------------------------------------------------
package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"sort"
	"sync/atomic"
	"time"
)

var gCert []byte

func initServer(showUI bool) {
	initServerUI(showUI)
}

func finiServer() {
	ui.fini()
	logFini()
}

func showAcceptedIPVersion() {
	var ipVerString = "ipv4, ipv6"
	if ipVer == ethrIPv4 {
		ipVerString = "ipv4"
	} else if ipVer == ethrIPv6 {
		ipVerString = "ipv6"
	}
	ui.printMsg("Accepting IP version: %s", ipVerString)
}

func runServer(serverParam ethrServerParam) {
	defer stopStatsTimer()
	initServer(serverParam.showUI)
	startStatsTimer()
	fmt.Println("-----------------------------------------------------------")
	showAcceptedIPVersion()
	ui.printMsg("Listening on port %d for TCP & UDP", gEthrPort)
	srvrRunUDPServer()
	err := srvrRunTCPServer()
	if err != nil {
		finiServer()
		fmt.Printf("Fatal error running TCP server: %v", err)
		os.Exit(1)
	}
}

func handshakeWithClient(test *ethrTest, conn net.Conn, buffer *bytes.Buffer) (testID EthrTestID, clientParam EthrClientParam, err error) {
	ethrMsg := &EthrMsg{}
	decoder := gob.NewDecoder(buffer)
	err = decoder.Decode(ethrMsg)
	if err != nil || ethrMsg.Type != EthrSyn {
		err = os.ErrInvalid
		return
	}
	testID = ethrMsg.Sync.TestID
	clientParam = ethrMsg.Syn.ClientParam
	delay := timeToNextTick()
	ethrMsg = createAckMsg(gCert, delay)
	var writeBuffer bytes.Buffer
	encoder := gob.NewEncoder(&writeBuffer)
	err = encoder.Encode(ethrMsg)
	if err != nil {
		ui.printErr("Failed to encode ACK message via Gob: %v", err)
	}
	_, err = conn.Write(writeBuffer.Bytes())
	if err != nil {
		ui.printErr("Failed to send ACK message back to Ethr client: %v", err)
	}
	writeBuffer.Reset()
	return
}

func srvrRunTCPServer() error {
	l, err := net.Listen(tcp(ipVer), hostAddr+":"+gEthrPortStr)
	if err != nil {
		return err
	}
	defer l.Close()
	for {
		conn, err := l.Accept()
		if err != nil {
			ui.printErr("Error accepting new TCP connection: %v", err)
			continue
		}
		go srvrHandleNewTcpConn(conn)
	}
}

func srvrHandleNewTcpConn(conn net.Conn) {
	defer conn.Close()

	server, port, err := net.SplitHostPort(conn.RemoteAddr().String())
	ethrUnused(server, port)
	if err != nil {
		ui.printDbg("RemoteAddr: Split host port failed: %v", err)
		return
	}
	lserver, lport, err := net.SplitHostPort(conn.LocalAddr().String())
	if err != nil {
		ui.printDbg("LocalAddr: Split host port failed: %v", err)
		return
	}
	ethrUnused(lserver, lport)
	ui.printDbg("New connection from %v, port %v to %v, port %v", server, port, lserver, lport)

	test, isNew := createOrGetTest(server, TCP, All)
	if test == nil {
		return
	}
	if isNew {
		ui.emitTestHdr()
	}

	// For CPS and ConnectionLatency tests, there is no deterministic way to know when
	// the test starts from the client side and when it ends. This defer function ensures
	// that test is not created/deleted repeatedly by doing a deferred deletion. If another
	// connection comes with-in 2s, then another reference would be taken on existing
	// test object and it won't be deleted by safeDeleteTest call. This also ensures,
	// test header is not printed repeatedly via emitTestHdr.
	// Note: Similar mechanism is used in UDP tests to handle test lifetime as well.
	defer func() {
		time.Sleep(2 * time.Second)
		safeDeleteTest(test)
	}()

	// Always increment CPS count and then check if the test is Bandwidth
	// etc. and handle those cases as well.
	atomic.AddUint64(&test.testResult.cps, 1)

	// TODO: Assuming max ethr message size as 1024 sent over gob.
	bufferBytes := make([]byte, 1024)
	n, err := conn.Read(bufferBytes)
	if err != nil {
		return
	}
	buffer := bytes.NewBuffer(bufferBytes[:n])
	testID, clientParam, err := handshakeWithClient(test, conn, buffer)
	if err != nil {
		return
	}

	if testID.Protocol == TCP {
		if testID.Type == Bandwidth {
			srvrRunTCPBandwidthTest(test, testParam, conn)
		} else if testID.Type == Latency {
			ui.emitLatencyHdr()
			srvrRunTCPLatencyTest(test, testParam, conn)
		}
	}
}

func srvrRunTCPBandwidthTest(test *ethrTest, clientParam EthrClientParam, conn net.Conn) {
	size := clientParam.BufferSize
	buff := make([]byte, size)
	for i := uint32(0); i < clientParam.BufferSize; i++ {
		buff[i] = byte(i)
	}
	for {
		var err error
		if clientParam.Reverse {
			_, err = conn.Write(buff)
		} else {
			_, err = io.ReadFull(conn, buff)
		}
		if err != nil {
			ui.printDbg("Error sending/receiving data on a connection for bandwidth test: %v", err)
			break
		}
		atomic.AddUint64(&test.testResult.bw, uint64(size))
	}
}

func srvrRunTCPLatencyTest(test *ethrTest, clientParam EthrClientParam, conn net.Conn) {
	bytes := make([]byte, clientParam.BufferSize)
	rttCount := clientParam.RttCount
	latencyNumbers := make([]time.Duration, rttCount)
	for {
		_, err := io.ReadFull(conn, bytes)
		if err != nil {
			ui.printDbg("Error receiving data for latency test: %v", err)
			return
		}
		for i := uint32(0); i < rttCount; i++ {
			s1 := time.Now()
			_, err = conn.Write(bytes)
			if err != nil {
				ui.printDbg("Error sending data for latency test: %v", err)
				return
			}
			_, err = io.ReadFull(conn, bytes)
			if err != nil {
				ui.printDbg("Error receiving data for latency test: %v", err)
				return
			}
			e2 := time.Since(s1)
			latencyNumbers[i] = e2
		}
		sum := int64(0)
		for _, d := range latencyNumbers {
			sum += d.Nanoseconds()
		}
		elapsed := time.Duration(sum / int64(rttCount))
		sort.SliceStable(latencyNumbers, func(i, j int) bool {
			return latencyNumbers[i] < latencyNumbers[j]
		})
		//
		// Special handling for rttCount == 1. This prevents negative index
		// in the latencyNumber index. The other option is to use
		// roundUpToZero() but that is more expensive.
		//
		rttCountFixed := rttCount
		if rttCountFixed == 1 {
			rttCountFixed = 2
		}
		atomic.SwapUint64(&test.testResult.latency, uint64(elapsed.Nanoseconds()))
		avg := elapsed
		min := latencyNumbers[0]
		max := latencyNumbers[rttCount-1]
		p50 := latencyNumbers[((rttCountFixed*50)/100)-1]
		p90 := latencyNumbers[((rttCountFixed*90)/100)-1]
		p95 := latencyNumbers[((rttCountFixed*95)/100)-1]
		p99 := latencyNumbers[((rttCountFixed*99)/100)-1]
		p999 := latencyNumbers[uint64(((float64(rttCountFixed)*99.9)/100)-1)]
		p9999 := latencyNumbers[uint64(((float64(rttCountFixed)*99.99)/100)-1)]
		ui.emitLatencyResults(
			test.session.remoteIP,
			protoToString(test.testParam.TestID.Protocol),
			avg, min, max, p50, p90, p95, p99, p999, p9999)
	}
}

func srvrRunUDPServer() error {
	udpAddr, err := net.ResolveUDPAddr(udp(ipVer), hostAddr+":"+gEthrPortStr)
	if err != nil {
		ui.printDbg("Unable to resolve UDP address: %v", err)
		return err
	}
	l, err := net.ListenUDP(udp(ipVer), udpAddr)
	if err != nil {
		ui.printDbg("Error listening on %s for UDP pkt/s tests: %v", gEthrPortStr, err)
		return err
	}
	//
	// We use NumCPU here instead of NumThreads passed from client. The
	// reason is that for UDP, there is no connection, so all packets come
	// on same CPU, so it isn't clear if there are any benefits to running
	// more threads than NumCPU(). TODO: Evaluate this in future.
	//
	for i := 0; i < runtime.NumCPU(); i++ {
		go srvrRunUDPPacketHandler(l)
	}
	return nil
}

func srvrRunUDPPacketHandler(conn *net.UDPConn) {
	// This local map aids in efficiency to look up a test based on client's IP
	// address. We could use createOrGetTest but that takes a global lock.
	tests := make(map[string]*ethrTest)
	// For UDP, allocate buffer that can accomodate largest UDP datagram.
	readBuffer := make([]byte, 64*1024)
	n, remoteIP, err := 0, new(net.UDPAddr), error(nil)

	// This function handles UDP tests that came from clients that are no longer
	// sending any traffic. This is poor man's garbage collection to ensure the
	// server doesn't end up printing dormant client related statistics as UDP
	// has no reliable way to detect if client is active or not.
	go func() {
		for {
			time.Sleep(200 * time.Millisecond)
			for k, v := range tests {
				ui.printDbg("Found Test from server: %v, time: %v", k, v.lastAccess)
				// At 200ms of no activity, mark the test in-active so stats stop
				// printing.
				if time.Since(v.lastAccess) > (200 * time.Millisecond) {
					v.isActive = false
				}
				// At 2s of no activity, delete the test by assuming that client
				// has stopped.
				if time.Since(v.lastAccess) > (2 * time.Second) {
					ui.printDbg("Deleting UDP test from server: %v, lastAccess: %v", k, v.lastAccess)
					safeDeleteTest(v)
					delete(tests, k)
				}
			}
		}
	}()
	for err == nil {
		n, remoteIP, err = conn.ReadFromUDP(readBuffer)
		if err != nil {
			ui.printDbg("Error receiving data from UDP for bandwidth test: %v", err)
			continue
		}
		ethrUnused(n)
		server, port, _ := net.SplitHostPort(remoteIP.String())
		test, found := tests[server]
		if !found {
			test, isNew := createOrGetTest(server, UDP, All)
			if test != nil {
				tests[server] = test
			}
			if isNew {
				buffer := bytes.NewBuffer(readBuffer[:n])
				testParam, err := handshakeWithClient(test, conn, buffer)
				if err != nil {
					return
				}
				ui.printDbg("Creating UDP test from server: %v, lastAccess: %v", server, time.Now())
				ui.emitTestHdr()
			}
		}
		if test != nil {
			test.lastAccess = time.Now()
			atomic.AddUint64(&test.testResult.pps, 1)
			atomic.AddUint64(&test.testResult.bw, uint64(n))
		} else {
			ui.printDbg("Unable to create test for UDP traffic on port %s from %s port %s", gEthrPortStr, server, port)
		}
	}
}
