//go:build !custom || inputs || inputs.watermeter

package all

import _ "github.com/influxdata/telegraf/plugins/inputs/watermeter" // register plugin
