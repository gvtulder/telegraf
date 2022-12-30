package watermeter

import (
	"fmt"
	"sync"
	"time"
	"math"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"

	"github.com/influxdata/telegraf/plugins/common/pigpiod"
)

type Watermeter struct {
	Address string
	Port    string
	Pin     int
	wg      sync.WaitGroup
	acc     telegraf.Accumulator

	pi      pigpiod.Pi
	quit    chan bool
}

func NewWatermeter() *Watermeter {
	return &Watermeter{}
}

const sampleConfig = `
	## pigpiod IP address
	address = "127.0.0.1"
	## pigpiod port
	port = "8888"
	## the GPIO pin number
	pin = 17
`

func (t *Watermeter) SampleConfig() string {
	return sampleConfig
}

func (t *Watermeter) Description() string {
	return "Read watermeter infrared sensor"
}

func (t *Watermeter) Gather(acc telegraf.Accumulator) error {
	return nil
}

func (t *Watermeter) Start(acc telegraf.Accumulator) error {
	t.acc = acc

	pi, err := pigpiod.Start("127.0.0.1", "8888")
	if err != nil {
		return fmt.Errorf("E! Error Could not open GPIO", err)
	}
	t.pi = pi

	pin := pi.GetPin(uint(t.Pin))
	err = pin.SetPullUpDown(pigpiod.PullDown)
	if err != nil {
		return fmt.Errorf("E! Error Could set pulldown", err)
	}

	// listen for ticks
	ticks := make(chan uint32)
	prevTick := uint32(0)
	cb, err := pin.AddCallback(pigpiod.EitherEdge, func(pi int, gpio uint, level bool, tick uint32) {
		tickDiff := uint32(0)
		if prevTick <= tick {
			tickDiff = uint32(tick) - prevTick
		} else {
			// wraparound
			tickDiff = (math.MaxUint32 - prevTick) + uint32(tick)
		}
		prevTick = tick
		// fmt.Printf("Pi: %d  Pin: %d  Level: %d  Tick: %d  TickDiff: %d\n", pi, gpio, level, tick, tickDiff)
		if tickDiff > 1000000 && level {
			ticks <- tickDiff
		}
	})
	if err != nil {
		return fmt.Errorf("E! Error Could not register callback", err)
	}

	t.wg.Add(1)
	t.quit = make(chan bool)

	go func () {
		for {
			select {
			case tickDiff := <-ticks:
				if tickDiff > 1000000 {
					fields := map[string]interface{}{
						"liters": 1,
					}
					tags := make(map[string]string)
					t.acc.AddFields("watermeter", fields, tags, time.Now())
				}
			case <-t.quit:
				cb.Cancel()
				pi.Stop()
				return
			}
		}
	}()


	return nil
}

func (t *Watermeter) Stop() {
	t.quit <- true
	t.wg.Wait()
}

func init() {
	inputs.Add("watermeter", func() telegraf.Input {
		return NewWatermeter()
	})
}
