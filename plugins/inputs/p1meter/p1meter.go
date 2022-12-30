package p1meter

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"strconv"
	"sync"
	"time"

	"github.com/npat-efault/crc16"
	"github.com/tarm/serial"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"
)

var crc16conf = &crc16.Conf{
	Poly: 0x8005, BitRev: true,
	IniVal: 0x0, FinVal: 0x0,
	BigEnd: true,
}

type P1Packet struct {
	header string
	eid string
	tariff int
	low_consumed int
	high_consumed int
	current_consumed int
	gas_consumed int
}

var fieldRegexp = regexp.MustCompile("^([-.:0-9]+)(?:\\(([^)]*)\\))+")
var kWhRegexp = regexp.MustCompile("^([.0-9]+)\\*kWh$")
var m3Regexp = regexp.MustCompile("^([.0-9]+)\\*m3$")

func updateField(packet *P1Packet, key string, value string) (parsingError error) {
	var kwhField *int
	var gasField *int
	switch key {
	case "0-0:96.1.1":
		packet.eid = value
	case "0-0:96.14.0":
		var err error
		packet.tariff, err = strconv.Atoi(value)
		if err != nil {
			parsingError = errors.New(fmt.Sprintf("could not parse tariff ('%s')", value))
		}
	case "1-0:1.8.1":
		kwhField = &packet.low_consumed
	case "1-0:1.8.2":
		kwhField = &packet.high_consumed
	case "1-0:1.7.0":
		kwhField = &packet.current_consumed
	case "1-1:24.2.1":
		gasField = &packet.gas_consumed
	case "1-2:24.2.1":
		gasField = &packet.gas_consumed
	case "1-3:24.2.1":
		gasField = &packet.gas_consumed
	case "1-4:24.2.1":
		gasField = &packet.gas_consumed
	}
	if kwhField != nil {
		v, err := strconv.ParseFloat(strings.Split(value, "*")[0], 64)
		if err != nil {
			parsingError = errors.New(fmt.Sprintf("error parsing kWh figure, '%s'", value))
		} else {
			*kwhField = int(v * 1000)
		}
	}
	if gasField != nil {
		v, err := strconv.ParseFloat(strings.Split(value, "*")[0], 64)
		if err != nil {
			parsingError = errors.New(fmt.Sprintf("error parsing m3 figure, '%s'", value))
		} else {
			*gasField = int(v)
		}
	}
	return parsingError
}

type P1Meter struct {
	Port string
	Baud int

	serialPort *serial.Port
	wg      sync.WaitGroup
	acc     telegraf.Accumulator

	sync.Mutex
}

func NewP1Meter() *P1Meter {
	return &P1Meter{}
}

const sampleConfig = `
	## the path to the USB port
	port = "/dev/ttyUSB0"

	## the baud rate
	baud = 115200
`

func (t *P1Meter) SampleConfig() string {
	return sampleConfig
}

func (t *P1Meter) Description() string {
	return "Read P1 meter telegrams from a serial port"
}

func (t *P1Meter) Gather(acc telegraf.Accumulator) error {
	return nil
}

func (t *P1Meter) Start(acc telegraf.Accumulator) error {
	t.Lock()
	defer t.Unlock()

	t.acc = acc

	c := &serial.Config{Name: t.Port, Baud: t.Baud}
	s, err := serial.OpenPort(c)
	if err != nil {
		t.acc.AddError(fmt.Errorf("E! Error Could not connect to serial port %s: %v", t.Port, err))
	}

	t.serialPort = s

	h := func(packet P1Packet) {
		fields := map[string]interface{}{
			"low_consumed": packet.low_consumed,
			"high_consumed": packet.high_consumed,
			"current_consumed": packet.current_consumed,
			"gas_consumed": packet.gas_consumed,
		}
		tags := map[string]string{
			"tarief": strconv.Itoa(packet.tariff),
		}
		t.acc.AddFields("p1meter", fields, tags, time.Now())
	}
	herr := func(err error) {
		t.acc.AddError(fmt.Errorf("E! Error P1 packet parsing error: %v", err))
	}

	// create a goroutine to handle this reader
	t.wg.Add(1)
	go t.runParsingLoop(s, h, herr)

	return nil
}

func (t *P1Meter) runParsingLoop(s io.Reader, handler func(P1Packet), errorHandler func(error)) {
	defer t.wg.Done()

	scanner := bufio.NewScanner(s)

	var curPacket P1Packet
	rawPacket := ""
	var parsingError error = nil
	reading := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) > 0 {
			if reading {
				if line[0] == '!' {
					// end token, checksum
					reading = false
					givenChecksum, _ := strconv.ParseUint(line[1:], 16, 16)
					computedChecksum := uint64(crc16.Checksum(crc16conf, []byte(rawPacket + "!")))
					if givenChecksum != computedChecksum {
						parsingError = errors.New(fmt.Sprintf("invalid checksum, given %x but computed %x", givenChecksum, computedChecksum))
					} else {
						// finished packet
						handler(curPacket)
					}
				} else if m := fieldRegexp.FindStringSubmatch(line); m != nil {
					// field with value
					parsingError = updateField(&curPacket, m[1], m[2])
				}
			} else if line[0] == '/' {
				// first line, header
				curPacket.header = line[1:]
				rawPacket = ""
				reading = true
				parsingError = nil
			}
		}
		if reading {
			rawPacket += line
			rawPacket += "\r\n"
		}
		if parsingError != nil {
			errorHandler(parsingError)
			parsingError = nil
			reading = false
		}
	}
}

func (t *P1Meter) Stop() {
	t.Lock()
	defer t.Unlock()

	err := t.serialPort.Close()
	if err != nil {
		t.acc.AddError(fmt.Errorf("E! Error stopping P1 reading on serial port %s: %v\n", t.Port, err))
	}
	t.wg.Wait()
}

func init() {
	inputs.Add("p1meter", func() telegraf.Input {
		return NewP1Meter()
	})
}
