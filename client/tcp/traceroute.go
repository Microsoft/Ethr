package tcp

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"

	"weavelab.xyz/ethr/client"
	"weavelab.xyz/ethr/client/payloads"
	"weavelab.xyz/ethr/ethr"
	"weavelab.xyz/ethr/session"
)

func (c Tests) TestTraceRoute(test *session.Test, gap time.Duration, mtrMode bool, maxHops int, results chan client.TestResult) {
	hops, err := c.discoverHops(test, mtrMode, maxHops)
	if err != nil {
		results <- client.TestResult{
			Success: false,
			Error:   fmt.Errorf("destination (%s) not responding to TCP connection", test.RemoteIP),
			Body:    payloads.TraceRoutePayload{Hops: hops},
		}
		return
	}
	if !mtrMode {
		results <- client.TestResult{
			Success: true,
			Error:   nil,
			Body:    payloads.TraceRoutePayload{Hops: hops},
		}
		return
	}
	for i := 0; i < len(hops); i++ {
		if hops[i].Addr != "" {
			go c.probeHops(test, gap, i, hops)
		}
	}
	results <- client.TestResult{
		Success: true,
		Error:   nil,
		Body:    payloads.TraceRoutePayload{Hops: hops},
	}
}

func (c Tests) probeHops(test *session.Test, gap time.Duration, hop int, hops []payloads.HopData) {
	seq := 0
	for {
		select {
		case <-test.Done:
			return
		default:
			t0 := time.Now()
			err, _ := c.probeHop(test, hop+1, hops[hop].Addr, &hops[hop])
			if err == nil {
			}
			seq++
			t1 := time.Since(t0)
			if t1 < gap {
				time.Sleep(gap - t1)
			}
		}
	}
}

func (c Tests) discoverHops(test *session.Test, mtrMode bool, maxHops int) ([]payloads.HopData, error) {
	hops := make([]payloads.HopData, maxHops)
	for i := 0; i < maxHops; i++ {
		var hopData payloads.HopData
		err, isLast := c.probeHop(test, i+1, "", &hopData)
		if err == nil {
			hopData.Name, hopData.FullName = lookupHopName(hopData.Addr)
		}
		//if hopData.Addr != "" {
		//	if mtrMode {
		//		c.Logger.Info("%2d.|--%s", i+1, hopData.Addr+" ["+hopData.FullName+"]")
		//	} else {
		//		c.Logger.Info("%2d.|--%-70s %s", i+1, hopData.Addr+" ["+hopData.FullName+"]", ui.DurationToString(hopData.Last))
		//	}
		//} else {
		//	c.Logger.Info("%2d.|--%s", i+1, "???")
		//}
		hops[i] = hopData
		if isLast {
			return hops[:i+1], nil
		}
	}
	return nil, os.ErrNotExist
}

func lookupHopName(addr string) (string, string) {
	name := ""
	tname := ""
	if addr == "" {
		return tname, name
	}
	names, err := net.LookupAddr(addr)
	if err == nil && len(names) > 0 {
		name = names[0]
		sz := len(name)

		if sz > 0 && name[sz-1] == '.' {
			name = name[:sz-1]
		}
		tname = name
		if len(name) > 16 {
			tname = name[:16] + "..."
		}
	}
	return tname, name
}

func (c Tests) probeHop(test *session.Test, hop int, hopIP string, hopData *payloads.HopData) (error, bool) {
	isLast := false
	icmpConn, err := c.NetTools.IcmpNewConn(test.RemoteIP)
	if err != nil {
		return fmt.Errorf("failed to create ICMP connection: %w", err), isLast
	}
	defer icmpConn.Close()
	localPortNum := uint16(8888)
	if c.NetTools.LocalPort != 0 {
		localPortNum = c.NetTools.LocalPort
	}
	localPortNum += uint16(hop)
	b := make([]byte, 4)
	binary.BigEndian.PutUint16(b[0:], localPortNum)
	remotePortNum, err := strconv.ParseUint(test.RemotePort, 10, 16)
	binary.BigEndian.PutUint16(b[2:], uint16(remotePortNum))
	peerAddrChan := make(chan string)
	endTimeChan := make(chan time.Time)
	go func() {
		peerAddr := ""
		// TODO have max messages?
		for {
			icmpMsg, peer, _ := c.NetTools.ReceiveICMPFromPeer(icmpConn, time.Second*2, hopIP)
			if icmpMsg.Type == ipv4.ICMPTypeTimeExceeded || icmpMsg.Type == ipv6.ICMPTypeTimeExceeded {
				body := icmpMsg.Body.(*icmp.TimeExceeded).Data
				index := bytes.Index(body, b[:4])
				if index > 0 {
					peerAddr = peer.String()
					break
				}
			}
		}

		endTimeChan <- time.Now()
		peerAddrChan <- peerAddr
	}()

	startTime := time.Now()
	var endTime time.Time
	peerAddr := ""

	// For TCP Traceroute an ICMP error message will be sent for everything except the last connection which
	// should establish correctly. The go routine above handles parsing the ICMP error into info used below.
	conn, err := c.NetTools.Dial(ethr.TCP, test.DialAddr, c.NetTools.LocalIP.String(), localPortNum, hop, 0)
	hopData.Sent++
	if err != nil { // majority case
		endTime = <-endTimeChan
		peerAddr = <-peerAddrChan
	} else {
		_ = conn.Close()
		endTime = time.Now()
		isLast = true
		peerAddr = test.RemoteIP
	}

	elapsed := endTime.Sub(startTime)
	if peerAddr == "" || (hopIP != "" && peerAddr != hopIP) {
		hopData.Lost++
		return fmt.Errorf("failed to complete connection or receive ICMP TTL Exceeded: %w", os.ErrNotExist), isLast
	}
	c.calcHopData(hopData, peerAddr, elapsed)
	return nil, isLast
}

func (c Tests) calcHopData(hopData *payloads.HopData, peerAddr string, elapsed time.Duration) {
	hopData.Addr = peerAddr
	hopData.Last = elapsed
	if hopData.Best > elapsed {
		hopData.Best = elapsed
	}
	if hopData.Worst < elapsed {
		hopData.Worst = elapsed
	}
	hopData.Total += elapsed
	hopData.Rcvd++
}
