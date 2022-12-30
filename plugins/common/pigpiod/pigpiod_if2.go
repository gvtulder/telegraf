package pigpiod

/*
#cgo CFLAGS: -pthread -W -Wall -Wno-unused-parameter -Wno-format-extra-args -Wbad-function-cast -Wno-unused-variable -O2 -g
#cgo LDFLAGS: -lpigpiod_if2 -lrt
#include <pigpiod_if2.h>

extern int goAddCallbackFunc(int pi, unsigned userGpio, unsigned edge, int cbi);
*/
import "C"
import (
  "fmt"
  "sync"
)

type Pi struct {
  id int
  connectionId string
  refCount int
}
type Pin struct {
  pi Pi
  gpio uint
}
type GpioMode uint
type PUD uint
type Edge uint
type Callback int

const (
  GpioModeInput  GpioMode = GpioMode(C.PI_INPUT)
  GpioModeOutput GpioMode = GpioMode(C.PI_OUTPUT)
  GpioModeAlt0   GpioMode = GpioMode(C.PI_ALT0)
  GpioModeAlt1   GpioMode = GpioMode(C.PI_ALT1)
  GpioModeAlt2   GpioMode = GpioMode(C.PI_ALT2)
  GpioModeAlt3   GpioMode = GpioMode(C.PI_ALT3)
  GpioModeAlt4   GpioMode = GpioMode(C.PI_ALT4)
  GpioModeAlt5   GpioMode = GpioMode(C.PI_ALT5)
)

const (
  PullOff  PUD = PUD(C.PI_PUD_OFF)
  PullDown PUD = PUD(C.PI_PUD_DOWN)
  PullUp   PUD = PUD(C.PI_PUD_UP)
)

const (
  RisingEdge  Edge = Edge(C.RISING_EDGE)
  FallingEdge Edge = Edge(C.FALLING_EDGE)
  EitherEdge  Edge = Edge(C.EITHER_EDGE)
)

func cError(errNo int) (err error) {
  err = fmt.Errorf("pigpiod_if2 error: %d", errNo)
  return
}

var piListMu sync.Mutex
var piList = make(map[string]Pi)

func Start(address string, port string) (pi Pi, err error) {
	piListMu.Lock()
	defer piListMu.Unlock()
  connectionId := address + ":" + port

  // reuse existing connection
  pi, piPresent := piList[connectionId]
  if piPresent {
    pi.refCount++
    return
  }

  // make new connection
  id := C.pigpio_start(C.CString(address), C.CString(port))
  if int(id) < 0 {
    err = fmt.Errorf("Failed to start pigpiod_if2 with address '%s' and port '%s'.", address, port)
  } else {
    pi = Pi{id: int(id), connectionId: connectionId, refCount: 1}
    piList[connectionId] = pi
  }
  return
}

func (pi Pi) Stop() {
	piListMu.Lock()
	defer piListMu.Unlock()
  pi.refCount--
  if pi.refCount == 0 {
    C.pigpio_stop(C.int(pi.id))
    delete(piList, pi.connectionId)
  }
}

func (pi Pi) GetPin(gpio uint) (pin Pin) {
  return Pin{pi: pi, gpio: gpio}
}

func (pin Pin) SetMode(mode GpioMode) (err error) {
  r := C.set_mode(C.int(pin.pi.id), C.uint(pin.gpio), C.uint(mode))
  if r < 0 {
    err = Errno(r)
  }
  return
}

func (pin Pin) GetMode() (mode GpioMode, err error) {
  r := C.get_mode(C.int(pin.pi.id), C.uint(pin.gpio))
  if r < 0 {
    err = Errno(r)
  } else {
    mode = GpioMode(r)
  }
  return
}

func (pin Pin) SetPullUpDown(pud PUD) (err error) {
  r := C.set_pull_up_down(C.int(pin.pi.id), C.uint(pin.gpio), C.uint(pud))
  if r < 0 {
    err = Errno(r)
  }
  return
}

func (pin Pin) GpioRead() (level bool, err error) {
  r := C.gpio_read(C.int(pin.pi.id), C.uint(pin.gpio))
  if r < 0 {
    err = Errno(r)
  } else {
    level = (r > 0)
  }
  return
}

func (pin Pin) GpioWrite(level bool) (err error) {
  l := 0
  if level {
    l = 1
  }
  r := C.gpio_write(C.int(pin.pi.id), C.uint(pin.gpio), C.uint(l))
  if r < 0 {
    err = Errno(r)
  }
  return
}

func (pin Pin) AddCallback(edge Edge, callbackFunc CallbackFunc) (callback Callback, err error) {
  cbi := registerCallbackFunc(callbackFunc)
  r := C.goAddCallbackFunc(C.int(pin.pi.id), C.uint(pin.gpio), C.uint(edge), C.int(cbi))
  if r < 0 {
    err = Errno(r)
  } else {
    callback = Callback(r)
  }
  return
}

func (callback Callback) Cancel() (err error) {
  r := C.callback_cancel(C.uint(callback))
  if r < 0 {
    err = Errno(r)
  }
  return
}

func Version() (version uint) {
  version = uint(C.pigpiod_if_version())
  return
}

