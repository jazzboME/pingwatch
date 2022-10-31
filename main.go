package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	probing "github.com/prometheus-community/pro-bing"
	"github.com/schollz/progressbar/v3"
)

type pktStat struct {
	seq  int
	time time.Time
}

var curPinging bool
var lastRecv, lastSend, lastBreak pktStat
var curPingRun int
var bar *progressbar.ProgressBar

func main() {
	silent := flag.Bool("s", false, "")
	flag.Parse()

	host := flag.Arg(0)
	pinger, err := probing.NewPinger(host)
	pinger.SetPrivileged(true)
	pinger.Debug = true
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	lastRecv.seq = -2
	lastRecv.time = time.Now()
	lastBreak.time = time.Now()

	bar = progressbar.NewOptions(-1,
		progressbar.OptionSetDescription("pingwatch"),
		progressbar.OptionShowCount(),
		progressbar.OptionSpinnerType(7),
		progressbar.OptionSetItsString("ping"),
	)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for range c {
			pinger.Stop()
		}
	}()

	pinger.OnRecv = func(pkt *probing.Packet) {
		curPinging = true
		if lastRecv.seq < pkt.Seq-1 && lastRecv.seq >= 0 {
			curTime := time.Now()
			duration := curTime.Sub(lastRecv.time)
			curPingRun = 0
			if !(*silent) {
				fmt.Printf("%s: Restoring after ping break from %d to %d (%s).\n",
					prettyTime(curTime), lastRecv.seq, pkt.Seq, duration)
				bar.Set(0)
				bar.Reset()
			}
			log.Printf("Restoring after ping break from %d to %d (%s).\n",
					lastRecv.seq, pkt.Seq, duration)
			lastBreak.seq = pkt.Seq
			lastBreak.time = curTime
		}
		if lastRecv.seq > pkt.Seq {
			curPingRun = 0
			if !(*silent) {
				fmt.Printf("\n%s: Received out of sequence ping, %d after %d.\n",
					prettyTime(time.Now()), pkt.Seq, lastRecv.seq)
				bar.Set(0)
				bar.Reset()
			}
			log.Printf("Received out of sequence ping, %d after %d.\n",
				pkt.Seq, lastRecv.seq)
		}
		if lastRecv.seq == pkt.Seq-1 {
			curPingRun++
			if !(*silent) {
				bar.Add(1)
			}
		}
		lastRecv = pktStat{seq: pkt.Seq, time: time.Now()}
	}

	pinger.OnSend = func(pkt *probing.Packet) {
		curTime := time.Now()
		duration := curTime.Sub(lastBreak.time)
		if (pkt.Seq > lastRecv.seq+1) && curPinging {
			if !(*silent) {
				fmt.Printf("\n%s: Sending but not receiving after continued connectivity of %s.\n", 
					prettyTime(curTime), duration)
			}
			log.Printf("Sending but not receiving after continued connectivity of %s\n", 
					duration)
			curPinging = false
		}
		lastSend = pktStat{seq: pkt.Seq, time: curTime}
	}

	pinger.OnFinish = func(stats *probing.Statistics) {
		if !(*silent) {
			fmt.Printf("\n--- %s ping statistics ---\n", stats.Addr)
			fmt.Printf("%d packets transmitted, %d packets received, %d duplicates, %v%% packet loss\n",
				stats.PacketsSent, stats.PacketsRecv, stats.PacketsRecvDuplicates, stats.PacketLoss)
			fmt.Printf("round-trip min/avg/max/stddev = %v/%v/%v/%v\n",
				stats.MinRtt, stats.AvgRtt, stats.MaxRtt, stats.StdDevRtt)
		}

		log.Printf("\n--- %s ping statistics ---\n", stats.Addr)
		log.Printf("%d packets transmitted, %d packets received, %d duplicates, %v%% packet loss\n",
			stats.PacketsSent, stats.PacketsRecv, stats.PacketsRecvDuplicates, stats.PacketLoss)
		log.Printf("round-trip min/avg/max/stddev = %v/%v/%v/%v\n",
			stats.MinRtt, stats.AvgRtt, stats.MaxRtt, stats.StdDevRtt)
	}

	if !(*silent) {
		fmt.Printf("PING %s (%s):\n", pinger.Addr(), pinger.IPAddr())
	}
	log.Printf("PING %s (%s):\n", pinger.Addr(), pinger.IPAddr())
	err = pinger.Run()
	if err != nil {
		fmt.Println("Failed to ping target host:", err)
	}
}

func prettyTime(t time.Time) string {
	return t.Format("Mon Jan 2, 2006 03:04:05.9 pm")
}
