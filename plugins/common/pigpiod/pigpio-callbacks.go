package pigpiod

/*
#cgo CFLAGS: -std=gnu99
#include <stdint.h>
#include <pigpiod_if2.h>
*/
import "C"

import "sync"

type CallbackFunc func(pi int, gpio uint, level bool, tick uint32)

var callbackFuncMu sync.Mutex
var callbackFuncs = make(map[int]CallbackFunc)
var callbackFuncIndex int

func lookupCallbackFunc(cbIndex int) CallbackFunc {
	callbackFuncMu.Lock()
	defer callbackFuncMu.Unlock()
	return callbackFuncs[cbIndex]
}

func registerCallbackFunc(callbackFunc CallbackFunc) int {
	if callbackFunc == nil {
		return -1
	}
	callbackFuncMu.Lock()
	defer callbackFuncMu.Unlock()
	callbackFuncIndex++
	for callbackFuncs[callbackFuncIndex] != nil {
		callbackFuncIndex++
	}
	callbackFuncs[callbackFuncIndex] = callbackFunc
	return callbackFuncIndex
}

//export goCallbackFunc
func goCallbackFunc(cbIndex C.int, pi C.int, gpio C.uint, level C.uint, tick C.uint32_t) {
	fn := lookupCallbackFunc(int(cbIndex))
	fn(int(pi), uint(gpio), uint(level) > 0, uint32(tick))
}
