package exec

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"

  "github.com/go-pigpio/pigpio/pigpio"
)

type DHT22 struct {
	Address string
	Port    string
	Pin     int
	wg      sync.WaitGroup
	acc     telegraf.Accumulator

	pi      pigpiod.Pi
	pin     pigpiod.Pin
	quit    chan bool
}

func NewDHT22() *DHT22 {
	return &DHT22{}
}

const sampleConfig = `
	## pigpiod IP address
	address = "127.0.0.1"
	## pigpiod port
	port = "8888"
	## the GPIO pin number
	pin = 22
`

func (t *DHT22) SampleConfig() string {
	return sampleConfig
}

func (t *DHT22) Description() string {
	return "Read DHT22 temperature and humidity sensor"
}

func (t *DHT22) Gather(acc telegraf.Accumulator) error {
	err := t.pin.SetPullUpDown(pigpiod.PullDown)
	if err != nil {
		return fmt.Errorf("E! Error Could set pulldown", err)
	}
	err = t.pin.GpioWrite(false)
	if err != nil {
		return fmt.Errorf("E! Error Could write to GPIO", err)
	}
	time.Sleep(1 * time.Millisecond)
	err = t.pin.SetMode(pigpiod.GpioModeInput)
	if err != nil {
		return fmt.Errorf("E! Error Could set GPIO mode", err)
	}
	return nil
}

func (t *DHT22) Start(acc telegraf.Accumulator) error {
	t.acc = acc

	pi, err := pigpiod.Start("127.0.0.1", "8888")
	if err != nil {
		return fmt.Errorf("E! Error Could not open GPIO", err)
	}
	t.pi = pi
	t.pin = pi.GetPin(uint(t.Pin))

	var state decodeState

	// listen for ticks
	ticks := make(chan uint32)
	prevTick := uint32(0)
	cb, err := t.pin.AddCallback(pigpiod.RisingEdge, func(pi int, gpio uint, level bool, tick uint32) {
		tickDiff := uint32(0)
		if prevTick <= tick {
			tickDiff = uint32(tick) - prevTick
		} else {
			// wraparound
			tickDiff = (math.MaxUint32 - prevTick) + uint32(tick)
		}
		prevTick = tick
		// fmt.Printf("Pi: %d  Pin: %d  Level: %d  Tick: %d  TickDiff: %d\n", pi, gpio, level, tick, tickDiff)
		ticks <- tickDiff
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
				if tickDiff > 10000 {
					state.in_code = true
					state.code = 0
					state.bits = -2
				} else {
					state.bits++
					if state.bits >= 1 {
						state.code <<= 1

						if tickDiff >= 60 && tickDiff <= 100 {
							// 0 bit
						} else if tickDiff > 100 && tickDiff < 150 {
							// 1 bit
							state.code += 1
						} else {
							// invalid bit
							state.in_code = false
						}

						if state.in_code {
							if state.bits == 40 {
								res := decodeDhtxx(state)
								if res.valid {
									fields := map[string]interface{}{
										"temperature": res.temperature,
										"humidity": res.humidity,
									}
									tags := make(map[string]string)
									t.acc.AddFields("dht22", fields, tags, time.Now())
								}
							}
						}
					}
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

func (t *DHT22) Stop() {
	t.quit <- true
	t.wg.Wait()
}

func init() {
	inputs.Add("dht22", func() telegraf.Input {
		return NewDHT22()
	})
}



type decodeState struct {
	in_code bool
	code uint64
	bits int
}

type decoded struct {
	temperature float64
	humidity float64
	valid bool
}


func decodeDhtxx(state decodeState) (res decoded) {
/*
      +-------+-------+
      | DHT11 | DHTXX |
      +-------+-------+
Temp C| 0-50  |-40-125|
      +-------+-------+
RH%   | 20-80 | 0-100 |
      +-------+-------+

         0      1      2      3      4
      +------+------+------+------+------+
DHT21 |check-| temp | temp | RH%  | RH%  |
DHT22 |sum   | LSB  | MSB  | LSB  | MSB  |
DHT33 |      |      |      |      |      |
DHT44 |      |      |      |      |      |
      +------+------+------+------+------+
*/
	// fmt.Printf("in_code: %t  bits: %d  code: %d\n", state.in_code, state.bits, state.code)

	var div float64
	var t float64
	var h float64

	var bytes [8]uint
	for i:=0; i<8; i++ {
		bytes[i] = uint(state.code >> uint(8 * i)) & 0xFF
		// fmt.Printf("byte[%d] = %d\n", i, bytes[i])
	}

	chksum := (bytes[1] + bytes[2] + bytes[3] + bytes[4]) & 0xFF
	// fmt.Printf("chksum: %d\n", chksum)

	valid := false
	if chksum == bytes[0] {
		valid = true
		h = ((float64)((bytes[4]<<8) + bytes[3]))/10.0
		if h > 110.0 {
			valid = false
		}
		if (bytes[2] & 128) > 0 {
			div = -10.0
		} else {
			div = 10.0
		}
		t = ((float64)(((bytes[2]&127)<<8) + bytes[1])) / div
		if (t < -50.0) || (t > 135.0) {
			valid = false
		}
	}

	if valid {
		res.temperature = math.Round(t * 10) / 10
		res.humidity = math.Round(h * 10) / 10
		res.valid = true
		// fmt.Printf("temperature: %f  humidity: %f\n", t, h)
	} else {
		res.valid = false
	}
	return
}
