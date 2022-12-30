//go:build !custom || inputs || inputs.p1meter

package all

import _ "github.com/influxdata/telegraf/plugins/inputs/p1meter" // register plugin
