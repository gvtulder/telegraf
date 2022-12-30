#include <stdlib.h>
#include <pigpiod_if2.h>
#include "_cgo_export.h"

void goCallbackFunc_cgo(int pi, unsigned gpio, unsigned level, uint32_t tick, void *userdata);
int goAddCallbackFunc(int pi, unsigned userGpio, unsigned edge, int cbi);

typedef struct {
    int cbi;
} goCallbackFunc_userdata;

void goCallbackFunc_cgo(int pi, unsigned gpio, unsigned level, uint32_t tick, void *userdata) {
    goCallbackFunc_userdata *myUserdata = (goCallbackFunc_userdata*)userdata;
    goCallbackFunc(myUserdata->cbi, pi, gpio, level, tick);
}

int goAddCallbackFunc(int pi, unsigned userGpio, unsigned edge, int cbi) {
    goCallbackFunc_userdata *myUserdata;
    myUserdata = malloc(sizeof(goCallbackFunc_userdata));
    myUserdata->cbi = cbi;
    return callback_ex(pi, userGpio, edge, goCallbackFunc_cgo, myUserdata);
}

int goCancelCallbackFunc(unsigned callback_id) {
    return callback_id;
}

