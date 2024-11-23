package main

import (
	"context"
	"fmt"
	"image/color"
	"time"

	"github.com/AllenDang/giu"
	"github.com/samber/lo"

	"github.com/256dpi/naos/pkg/ble"
	"github.com/256dpi/naos/pkg/mdns"
	"github.com/256dpi/naos/pkg/msg"
)

var bleDiscover context.Context
var bleDiscoverCancel context.CancelFunc

var mdnsDiscover bool
var mdnsCache = map[string]bool{}

var devices []*managedDevice

func loop() {
	giu.Window("NAOS").Flags(giu.WindowFlagsMenuBar).Size(800, 600).Layout(
		giu.MenuBar().Layout(
			giu.Menu("Discover").Layout(
				giu.MenuItem("BLE").Selected(bleDiscover != nil).OnClick(func() {
					if bleDiscover != nil {
						bleDiscoverCancel()
						bleDiscover = nil
					} else {
						bleDiscover, bleDiscoverCancel = context.WithCancel(context.Background())
						go ble.Discover(bleDiscover, func(device msg.Device) {
							devices = append(devices, newManagedDevice(device))
						})
					}
				}),
				giu.MenuItem(("mDNS")).Selected(mdnsDiscover).OnClick(func() {
					if mdnsDiscover {
						mdnsDiscover = false
					} else {
						mdnsDiscover = true
						go func() {
							for mdnsDiscover {
								locs, _ := mdns.Discover(time.Second)
								for _, loc := range locs {
									if !mdnsCache[loc.Address] {
										mdnsCache[loc.Address] = true
										devices = append(devices, newManagedDevice(msg.NewHTTPDevice(loc.Address)))
									}
								}
							}
						}()
					}
				}),
			),
		),
		giu.Table().Columns(
			giu.TableColumn("ID"),
			giu.TableColumn("Active"),
			giu.TableColumn("Name"),
			giu.TableColumn("Type"),
			giu.TableColumn("Firmware Version"),
			giu.TableColumn("Actions"),
		).Rows(
			lo.Map(devices, func(device *managedDevice, _ int) *giu.TableRowWidget {
				active := device.Device.Active()
				return giu.TableRow(
					// giu.Selectable(device.ID()).Selected(selected).OnClick(func() {
					// 	selected = !selected
					// }).Flags(giu.SelectableFlagsSpanAllColumns),
					giu.Label(device.Device.Device().ID()),
					giu.Checkbox("", &active).OnChange(func() {
						if active {
							err := device.Device.Activate()
							if err != nil {
								fmt.Println(err)
							}
						} else {
							device.Device.Deactivate()
						}
					}),
					giu.Label(device.GetString("device-name")),
					giu.Label(device.GetString("device-type")),
					giu.Label(device.GetString("device-version")),
					giu.Button("Refresh").Disabled(!device.Device.Active()).OnClick(func() {
						err := device.Refresh()
						if err != nil {
							fmt.Println(err)
						}
					}),
				)
			})...,
		),
	)
}

func main() {
	wnd := giu.NewMasterWindow("NAOS", 800, 600, giu.MasterWindowFlagsFrameless|giu.MasterWindowFlagsTransparent)
	wnd.SetBgColor(color.Transparent)
	wnd.Run(loop)
}
