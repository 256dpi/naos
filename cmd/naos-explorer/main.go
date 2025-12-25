package main

import (
	"context"
	"flag"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/256dpi/naos/pkg/ble"
	"github.com/256dpi/naos/pkg/mdns"
	"github.com/256dpi/naos/pkg/mqtt"
	"github.com/256dpi/naos/pkg/msg"
	"github.com/256dpi/naos/pkg/serial"
)

var mqttURI = flag.String("mqtt", "", "The MQTT broker URI.")

func main() {
	// parse flags
	flag.Parse()

	// prepare state
	state := newState()

	// start BLE discovery
	go func() {
		for {
			err := ble.Discover(context.Background(), func(d ble.Description) {
				state.register(ble.NewDevice(d.Address))
			})
			if err != nil {
				state.log("[red]BLE discover error[-]: %v", err)
			}
		}
	}()

	// start mDNS discovery
	go func() {
		for {
			locs, err := mdns.Discover(time.Second)
			if err != nil {
				state.log("[red]mDNS discover error[-]: %v", err)
			} else {
				for _, loc := range locs {
					state.register(msg.NewHTTPDevice(loc.Hostname))
				}
			}
			time.Sleep(5 * time.Second)
		}
	}()

	// start serial discovery
	go func() {
		for {
			ports, err := serial.ListPorts()
			if err != nil {
				state.log("[red]Serial list error[-]: %v", err)
			} else {
				sort.Strings(ports)
				for _, port := range ports {
					dev, err := serial.NewDevice(port)
					if err != nil {
						state.log("[red]Serial open %s failed[-]: %v", port, err)
						continue
					}
					state.register(dev)
				}
			}
			time.Sleep(5 * time.Second)
		}
	}()

	// start MQTT discovery
	if *mqttURI != "" {
		go func() {
			for {
				router, err := mqtt.Connect(*mqttURI, "naos-explorer", 0)
				if err != nil {
					state.log("[red]MQTT connect error[-]: %v", err)
					time.Sleep(5 * time.Second)
					continue
				}
				err = mqtt.Discover(context.Background(), router, func(d mqtt.Description) {
					state.registerWithMeta(mqtt.NewDevice(router, d.BaseTopic), d.DeviceName, d.AppName, d.AppVersion)
				})
				if err != nil {
					state.log("[red]MQTT discover error[-]: %v", err)
				}
				time.Sleep(5 * time.Second)
			}
		}()
	}

	// run UI
	runUI(state)
}

func runUI(state *state) {
	// create app
	app := tview.NewApplication().
		EnableMouse(true)

	// set up pages
	pages := tview.NewPages()
	app.SetRoot(pages, true)

	// prepare table view
	table := tview.NewTable().
		SetSelectable(true, false)
	table.SetBorder(true).
		SetTitle("Devices")

	// prepare log view
	log := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(false)
	log.SetBorder(true).
		SetTitle("Logs (l to focus)")

	// prepare container
	container := tview.NewFlex().SetDirection(tview.FlexRow)
	container.AddItem(table, 0, 3, true)
	container.AddItem(log, 0, 1, false)

	// add main page
	pages.AddPage("main", container, true, true)

	// prepare log updater
	updateLogView := func() {
		lines := state.logger.Snapshot()
		if len(lines) == 0 {
			lines = []string{"No log messages yet"}
		}
		log.SetText(strings.Join(lines, "\n"))
		log.ScrollToEnd()
	}

	// update log immediately and on changes
	updateLogView()
	state.logger.Bind(func() {
		app.QueueUpdateDraw(updateLogView)
	})

	// prepare local devices
	var devices []*device

	// prepare table view updater
	updateTableView := func() {
		// capture selection and clear table
		currentRow, _ := table.GetSelection()
		table.Clear()

		// set headers
		headers := []string{"ID", "Device Name", "App Name", "App Version", "Age"}
		for col, h := range headers {
			cell := tview.NewTableCell(fmt.Sprintf("[yellow]%s", h)).SetSelectable(false)
			table.SetCell(0, col, cell)
		}

		// update devices
		devices = state.snapshot()

		// add devices
		for i, device := range devices {
			row := i + 1 // offset for header

			// device name (use ID as fallback)
			deviceName := device.DeviceName()
			if deviceName == "" {
				deviceName = "-"
			}

			// app name
			appName := device.AppName()
			if appName == "" {
				appName = "-"
			}

			// app version
			appVersion := device.AppVersion()
			if appVersion == "" {
				appVersion = "-"
			}

			table.SetCell(row, 0, tview.NewTableCell(device.ID()))
			table.SetCell(row, 1, tview.NewTableCell(deviceName))
			table.SetCell(row, 2, tview.NewTableCell(appName))
			table.SetCell(row, 3, tview.NewTableCell(appVersion))
			table.SetCell(row, 4, tview.NewTableCell(humanDuration(time.Since(device.LastSeen()))))
		}

		// restore selection
		if len(devices) > 0 {
			if currentRow < 1 {
				currentRow = 1
			}
			if currentRow > len(devices) {
				currentRow = len(devices)
			}
			table.Select(currentRow, 0)
		}
	}

	// update table immediately and every second
	updateTableView()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	go func() {
		for range ticker.C {
			app.QueueUpdateDraw(updateTableView)
		}
	}()

	// set selection handler
	table.SetSelectedFunc(func(row, col int) {
		// adjust for header row
		index := row - 1

		// check index
		if index < 0 || index >= len(devices) {
			return
		}

		// get device
		device := devices[index]

		// log action
		state.log("Activating: %s", device.ID())

		// create progress modal
		closeProgress := showProgressModal(app, pages, fmt.Sprintf("Activating\n%s...", device.ID()))

		// activate in background
		go func() {
			// activate device
			err := device.Activate()

			// continue in foreground
			app.QueueUpdateDraw(func() {
				// close progress
				closeProgress()

				// handle error
				if err != nil {
					state.log("[red]Activate failed for %s[-]: %v", device.ID(), err)
					showErrorModal(app, pages, fmt.Sprintf("Activate failed: %v", err))
					app.SetFocus(table)
					return
				}

				// log success
				state.log("Activated %s", device.ID())

				// open dashboard
				pageName := fmt.Sprintf("device-%s", device.ID())
				dash := newDashboard(app, pages, device, func() {
					// deactivate device
					device.Deactivate()

					// remove page and return to main
					app.QueueUpdateDraw(func() {
						pages.RemovePage(pageName)
						pages.SwitchToPage("main")
						app.SetFocus(table)
					})
				})

				// show dashboard
				pages.AddPage(pageName, dash.Root(), true, true)
				app.SetFocus(dash.DefaultFocus())
			})
		}()
	})

	// handle focus switching
	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRune {
			switch event.Rune() {
			case 'l', 'L':
				app.SetFocus(log)
				return nil
			}
		}
		return event
	})
	log.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			app.SetFocus(table)
			return nil
		}
		if event.Key() == tcell.KeyRune {
			switch event.Rune() {
			case 'l', 'L':
				app.SetFocus(table)
				return nil
			}
		}
		return event
	})

	// handle app done
	table.SetDoneFunc(func(key tcell.Key) {
		app.Stop()
	})

	// run app
	err := app.Run()
	if err != nil {
		panic(err)
	}
}
