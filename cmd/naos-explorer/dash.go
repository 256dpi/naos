package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/256dpi/naos/pkg/msg"
)

type dashboard struct {
	app         *tview.Application
	pages       *tview.Pages
	device      *device
	root        *tview.Flex
	infoView    *tview.TextView
	statusView  *tview.TextView
	paramTable  *tview.Table
	metricTable *tview.Table
	fsTree      *tview.TreeView
	fsPreview   *tview.TextView
	focusable   []tview.Primitive
	focusIndex  int
	paramRows   []paramRow
	metricRows  []metricRow
	done        chan struct{}
	onClose     func()
	logger      *logger

	// coredump state
	coredumpSize   uint32
	coredumpReason string

	// log streaming state
	logDone chan struct{}
}

type paramRow struct {
	info   msg.ParamInfo
	update msg.ParamUpdate
	value  string
}

type metricRow struct {
	info   msg.MetricInfo
	layout msg.MetricLayout
	value  string
	err    error
}

type fsNode struct {
	path   string
	info   *msg.FSInfo
	loaded bool
}

func newDashboard(app *tview.Application, pages *tview.Pages, device *device, onClose func()) *dashboard {
	dash := &dashboard{
		app:     app,
		pages:   pages,
		device:  device,
		done:    make(chan struct{}),
		onClose: onClose,
		logger:  newLogger(50),
	}

	dash.buildUI()
	dash.start()
	dash.updateInfo()

	return dash
}

func (d *dashboard) buildUI() {
	// create info view
	d.infoView = tview.NewTextView().
		SetDynamicColors(true)
	d.infoView.SetBorder(true).
		SetTitle("Device")

	// create status view
	d.statusView = tview.NewTextView().
		SetDynamicColors(true).
		SetText("(Tab) Switch  (F) Reload Dir  (U) Firmware  (C) Coredump  (D) Del Coredump  (L) Log  (Esc) Close")
	d.statusView.SetBorder(true).
		SetTitle("Status")

	// create parameter table
	d.paramTable = tview.NewTable().
		SetSelectable(true, false)
	d.paramTable.SetBorder(true).
		SetTitle("Parameters")
	d.paramTable.SetSelectedFunc(func(row, _ int) {
		d.editParam(row - 1)
	})

	// create metric table
	d.metricTable = tview.NewTable().
		SetSelectable(true, false)
	d.metricTable.SetBorder(true).
		SetTitle("Metrics")
	d.metricTable.SetSelectedFunc(func(row, _ int) {
		d.showMetricDetails(row - 1)
	})

	// create FS tree view
	d.fsTree = tview.NewTreeView()
	d.fsTree.SetBorder(true).
		SetTitle("File System")
	d.fsTree.SetSelectedFunc(func(node *tview.TreeNode) {
		d.handleTreeSelect(node)
	})

	// create FS preview
	d.fsPreview = tview.NewTextView().
		SetDynamicColors(true)
	d.fsPreview.SetBorder(true).
		SetTitle("Preview")
	d.fsPreview.SetWrap(true)

	// prepare log view
	log := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(false)
	log.SetBorder(true)

	// create top container
	top := tview.NewFlex().
		AddItem(d.infoView, 0, 2, false).
		AddItem(d.statusView, 0, 3, false)

	// create FS container
	fsPane := tview.NewFlex().
		AddItem(d.fsTree, 0, 1, true).
		AddItem(d.fsPreview, 0, 1, false)

	// create right container
	right := tview.NewFlex().
		SetDirection(tview.FlexRow)
	right.AddItem(d.metricTable, 0, 1, false)
	right.AddItem(fsPane, 0, 2, true)

	// create body container
	body := tview.NewFlex().
		AddItem(d.paramTable, 0, 2, true).
		AddItem(right, 0, 3, false)

	// create root container
	d.root = tview.NewFlex().
		SetDirection(tview.FlexRow)
	d.root.AddItem(top, 7, 0, false)
	d.root.AddItem(body, 0, 1, true)
	d.root.AddItem(log, 10, 0, false)

	// create focusable
	d.focusable = []tview.Primitive{d.paramTable, d.metricTable, d.fsTree}
	d.focusIndex = 0

	// handle input
	d.root.SetInputCapture(d.capture)

	// set up FS tree
	rootNode := tview.NewTreeNode("/").
		SetReference(&fsNode{path: "/", info: &msg.FSInfo{Name: "/", IsDir: true}}).
		SetExpanded(true)
	d.fsTree.SetRoot(rootNode)
	d.fsTree.SetCurrentNode(rootNode)

	// prepare log updater
	updateLogView := func() {
		lines := d.logger.Snapshot()
		if len(lines) == 0 {
			lines = []string{"No log messages yet"}
		}
		log.SetText(strings.Join(lines, "\n"))
		log.ScrollToEnd()
	}

	// update log immediately and on changes
	updateLogView()
	d.logger.Bind(func() {
		d.app.QueueUpdateDraw(updateLogView)
	})
}

func (d *dashboard) start() {
	// run background loops
	go d.loopParams()
	go d.loopMetrics()
	go d.loopCoredump()
	go d.loadDirectory(d.fsTree.GetRoot(), true)
}

func (d *dashboard) capture(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEscape:
		d.close()
		return nil
	case tcell.KeyTAB:
		d.focusIndex = (d.focusIndex + 1) % len(d.focusable)
		d.app.SetFocus(d.focusable[d.focusIndex])
		return nil
	case tcell.KeyBacktab:
		d.focusIndex = (d.focusIndex - 1 + len(d.focusable)) % len(d.focusable)
		d.app.SetFocus(d.focusable[d.focusIndex])
		return nil
	case tcell.KeyRune:
		switch event.Rune() {
		case 'f':
			d.reloadCurrentDir()
			return nil
		case 'u':
			d.promptFirmwareUpdate()
			return nil
		case 'c':
			d.downloadCoredump()
			return nil
		case 'd':
			d.deleteCoredump()
			return nil
		case 'l':
			d.toggleLogStreaming()
			return nil
		}
	default:
	}
	return event
}

func (d *dashboard) close() {
	// stop log streaming if active
	d.stopLogStreaming()

	close(d.done)
	if d.onClose != nil {
		go d.onClose()
	}
}

func (d *dashboard) log(format string, args ...any) {
	// log message
	d.logger.Append(format, args...)
}

func (d *dashboard) Root() tview.Primitive {
	return d.root
}

func (d *dashboard) DefaultFocus() tview.Primitive {
	if len(d.focusable) > 0 {
		return d.focusable[0]
	}
	return d.root
}

func (d *dashboard) loopParams() {
	// refresh params immediately and then periodically
	d.refreshParams()
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-d.done:
			return
		case <-ticker.C:
			d.refreshParams()
		}
	}
}

func (d *dashboard) refreshParams() {
	// get param service
	ps := d.device.ParamsService()

	// collect params
	err := ps.Collect()
	if err != nil {
		d.log("[red]Collect params failed[-]: %v", err)
		return
	}

	// prepare rows
	var rows []paramRow
	for info, update := range ps.All() {
		rows = append(rows, paramRow{
			info:   info,
			update: update,
			value:  formatParamValue(info, update),
		})
	}

	// set rows
	d.paramRows = rows

	// queue update
	d.queue(func() {
		d.renderParamRows(rows)
		d.updateInfo()
	})
}

func (d *dashboard) loopMetrics() {
	// refresh metrics immediately and then periodically
	d.refreshMetrics()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-d.done:
			return
		case <-ticker.C:
			d.refreshMetrics()
		}
	}
}

func (d *dashboard) refreshMetrics() {
	// get metrics service
	ms := d.device.MetricsService()

	// prepare rows
	var rows []metricRow
	for info, layout := range ms.All() {
		var values []float64
		var err error
		values, err = ms.Read(info.Name)
		formatted := formatMetricValues(values)
		rows = append(rows, metricRow{
			info:   info,
			layout: layout,
			value:  formatted,
			err:    err,
		})
		if err != nil {
			d.log("[red]Metric %s read failed[-]: %v", info.Name, err)
		}
	}

	// set rows
	d.metricRows = rows

	// queue update
	d.queue(func() {
		d.renderMetricRows(rows)
		d.updateInfo()
	})
}

func (d *dashboard) renderParamRows(rows []paramRow) {
	// clear table
	d.paramTable.Clear()

	// set headers
	headers := []string{"Name", "Mode", "Type", "Value", "Age"}
	for col, h := range headers {
		cell := tview.NewTableCell(fmt.Sprintf("[yellow]%s", h)).SetSelectable(false)
		d.paramTable.SetCell(0, col, cell)
	}

	// set rows
	for i, row := range rows {
		d.paramTable.SetCell(i+1, 0, tview.NewTableCell(row.info.Name))
		d.paramTable.SetCell(i+1, 1, tview.NewTableCell(paramModeShort(row.info.Mode)))
		d.paramTable.SetCell(i+1, 2, tview.NewTableCell(paramTypeString(row.info.Type)))
		d.paramTable.SetCell(i+1, 3, tview.NewTableCell(row.value))
		d.paramTable.SetCell(i+1, 4, tview.NewTableCell(fmt.Sprintf("%d", row.update.Age)))
	}
}

func (d *dashboard) renderMetricRows(rows []metricRow) {
	// clear table
	d.metricTable.Clear()

	// set headers
	headers := []string{"Name", "Kind", "Type", "Layout", "Latest"}
	for col, h := range headers {
		d.metricTable.SetCell(0, col, tview.NewTableCell(fmt.Sprintf("[yellow]%s", h)).SetSelectable(false))
	}

	// set rows
	for i, row := range rows {
		layoutDesc := layoutSummary(row.layout)
		latest := row.value
		if row.err != nil {
			latest = fmt.Sprintf("error: %v", row.err)
		}
		d.metricTable.SetCell(i+1, 0, tview.NewTableCell(row.info.Name))
		d.metricTable.SetCell(i+1, 1, tview.NewTableCell(metricKindString(row.info.Kind)))
		d.metricTable.SetCell(i+1, 2, tview.NewTableCell(metricTypeString(row.info.Type)))
		d.metricTable.SetCell(i+1, 3, tview.NewTableCell(layoutDesc))
		d.metricTable.SetCell(i+1, 4, tview.NewTableCell(latest))
	}
}

func (d *dashboard) editParam(row int) {
	// check selection
	if row < 0 || row >= len(d.paramRows) {
		return
	}

	// capture param info
	info := d.paramRows[row].info
	current := d.paramRows[row].value

	// ignore locked params
	if (info.Mode & msg.ParamModeLocked) != 0 {
		return
	}

	// trigger actions right away
	if info.Type == msg.ParamTypeAction {
		go d.writeParam(info, "")
		return
	}

	// toggle booleans right away
	if info.Type == msg.ParamTypeBool {
		newValue := "0"
		if current == "<False>" {
			newValue = "1"
		}
		go d.writeParam(info, newValue)
		return
	}

	// create form
	form := tview.NewForm()
	input := tview.NewInputField().
		SetText(current)
	form.AddFormItem(input)
	form.AddButton("Save", func() {
		text := input.GetText()
		go d.writeParam(info, text)
		d.pages.RemovePage("param-editor")
	})
	form.AddButton("Cancel", func() {
		d.pages.RemovePage("param-editor")
	})
	form.SetBorder(true).
		SetTitle(fmt.Sprintf("Set %s (%s)", info.Name, paramTypeString(info.Type)))
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			d.pages.RemovePage("param-editor")
			return nil
		}
		return event
	})

	// show form
	d.pages.AddPage("param-editor", centered(60, 10, form), true, true)
	d.app.SetFocus(form)
}

func (d *dashboard) writeParam(info msg.ParamInfo, text string) {
	// get param service
	ps := d.device.ParamsService()

	// write value
	err := ps.Set(info.Name, []byte(text))
	if err != nil {
		d.log("[red]Write %s failed[-]: %v", info.Name, err)
		return
	}

	// refresh params
	d.refreshParams()

	// handle success
	d.log("Parameter %s updated", info.Name)
}

func (d *dashboard) showMetricDetails(row int) {
	if row < 0 || row >= len(d.metricRows) {
		return
	}
	metric := d.metricRows[row]
	modal := tview.NewTextView().SetDynamicColors(true)
	modal.SetBorder(true).SetTitle(metric.info.Name)
	_, _ = fmt.Fprintf(modal, "Kind: %s\n", metricKindString(metric.info.Kind))
	_, _ = fmt.Fprintf(modal, "Type: %s\n", metricTypeString(metric.info.Type))
	_, _ = fmt.Fprintf(modal, "Values: %s\n", metric.value)
	_, _ = fmt.Fprintf(modal, "Layout: %s\n", layoutSummary(metric.layout))
	d.pages.AddPage("metric-details", centered(60, 12, modal), true, true)
	d.app.SetFocus(modal)
	modal.SetDoneFunc(func(key tcell.Key) {
		d.pages.RemovePage("metric-details")
		d.app.SetFocus(d.metricTable)
	})
}

func (d *dashboard) handleTreeSelect(node *tview.TreeNode) {
	if node == nil {
		return
	}
	ref, ok := node.GetReference().(*fsNode)
	if !ok || ref.info == nil {
		return
	}
	if ref.info.IsDir {
		node.SetExpanded(!node.IsExpanded())
		if !ref.loaded {
			d.loadDirectory(node, false)
		}
		return
	}
	d.previewFile(ref.path)
}

func (d *dashboard) loadDirectory(node *tview.TreeNode, force bool) {
	if node == nil {
		return
	}
	ref, ok := node.GetReference().(*fsNode)
	if !ok || ref.info == nil || !ref.info.IsDir {
		return
	}
	if ref.loaded && !force {
		return
	}
	nodePath := ref.path

	go func() {
		var entries []msg.FSInfo
		err := d.device.WithSession(func(s *msg.Session) error {
			list, err := msg.ListDir(s, nodePath, 5*time.Second)
			if err != nil {
				return err
			}
			entries = list
			return nil
		})
		if err != nil {
			d.log("[red]List %s failed[-]: %v", nodePath, err)
			return
		}
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].IsDir == entries[j].IsDir {
				return entries[i].Name < entries[j].Name
			}
			return entries[i].IsDir && !entries[j].IsDir
		})
		d.queue(func() {
			node.ClearChildren()
			for _, entry := range entries {
				entryCopy := entry
				childPath := pathJoin(nodePath, entryCopy.Name)
				child := tview.NewTreeNode(entryCopy.Name).SetReference(&fsNode{path: childPath, info: &entryCopy})
				if entryCopy.IsDir {
					child.SetColor(tcell.ColorYellow)
				}
				node.AddChild(child)
			}
			ref.loaded = true
		})
	}()
}

func (d *dashboard) reloadCurrentDir() {
	node := d.fsTree.GetCurrentNode()
	if node == nil {
		node = d.fsTree.GetRoot()
	}
	if node != nil {
		ref, _ := node.GetReference().(*fsNode)
		if ref != nil {
			ref.loaded = false
		}
	}
	d.loadDirectory(node, true)
}

func (d *dashboard) previewFile(path string) {
	const maxPreview = 8 * 1024
	go func() {
		var content []byte
		err := d.device.WithSession(func(s *msg.Session) error {
			info, err := msg.StatPath(s, path, 5*time.Second)
			if err != nil {
				return err
			}
			length := info.Size
			if length > maxPreview {
				length = maxPreview
			}
			data, err := msg.ReadFileRange(s, path, 0, length, nil, 5*time.Second)
			if err != nil {
				return err
			}
			content = data
			return nil
		})
		if err != nil {
			d.log("[red]Read %s failed[-]: %v", path, err)
			return
		}
		text := previewFile(content)
		d.queue(func() {
			d.fsPreview.SetText(fmt.Sprintf("%s\n\n%s", path, text))
		})
	}()
}

func (d *dashboard) promptFirmwareUpdate() {
	form := tview.NewForm()
	input := tview.NewInputField().SetLabel("Image Path").SetFieldWidth(50)
	form.AddFormItem(input)
	form.AddButton("Start", func() {
		d.pages.RemovePage("fw-update")
		imagePath := strings.TrimSpace(input.GetText())
		if imagePath == "" {
			d.log("[red]Firmware update aborted[-]: missing image path")
			return
		}
		go d.performFirmwareUpdate(imagePath)
	})
	form.AddButton("Cancel", func() {
		d.pages.RemovePage("fw-update")
	})
	form.SetBorder(true).SetTitle("Firmware Update")
	d.pages.AddPage("fw-update", centered(70, 7, form), true, true)
	d.app.SetFocus(form)
}

func (d *dashboard) performFirmwareUpdate(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		d.log("[red]Read firmware image failed[-]: %v", err)
		return
	}
	total := len(data)
	d.log("Firmware update started: %s (%d bytes)", path, total)
	err = d.device.WithSession(func(s *msg.Session) error {
		return msg.Update(s, data, func(done int) {
			d.log("Firmware update progress: %d / %d bytes (%.2f%%)", done, total, float64(done)/float64(total)*100)
		}, 60*time.Second)
	})
	if err != nil {
		d.log("[red]Firmware update failed[-]: %v", err)
		return
	}
	d.log("Firmware update complete")
}

func (d *dashboard) updateInfo() {
	// format coredump status
	coredumpStatus := "None"
	if d.coredumpSize > 0 {
		coredumpStatus = fmt.Sprintf("%d bytes", d.coredumpSize)
		if d.coredumpReason != "" {
			coredumpStatus += fmt.Sprintf(" (%s)", d.coredumpReason)
		}
	}

	// format log status
	logStatus := "Off"
	if d.logDone != nil {
		logStatus = "[green]Streaming[-]"
	}

	text := fmt.Sprintf("[yellow]ID:[-] %s\n[yellow]Parameters:[-] %d\n[yellow]Metrics:[-] %d\n[yellow]Coredump:[-] %s\n[yellow]Log:[-] %s",
		d.device.ID(), len(d.paramRows), len(d.metricRows), coredumpStatus, logStatus)
	d.infoView.SetText(text)
}

func (d *dashboard) queue(fn func()) {
	go d.app.QueueUpdateDraw(fn)
}

func (d *dashboard) loopCoredump() {
	// check coredump immediately and then periodically
	d.checkCoredump()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-d.done:
			return
		case <-ticker.C:
			d.checkCoredump()
		}
	}
}

func (d *dashboard) checkCoredump() {
	var size uint32
	var reason string
	err := d.device.WithSession(func(s *msg.Session) error {
		var err error
		size, reason, err = msg.CheckCoredump(s, 5*time.Second)
		return err
	})
	if err != nil {
		d.log("[red]Check coredump failed[-]: %v", err)
		return
	}

	// update state and info
	d.coredumpSize = size
	d.coredumpReason = reason
	d.queue(func() {
		d.updateInfo()
	})
}

func (d *dashboard) downloadCoredump() {
	// check if coredump is available
	if d.coredumpSize == 0 {
		d.log("No coredump available")
		return
	}

	// create form for file path
	form := tview.NewForm()
	input := tview.NewInputField().SetLabel("Save Path").SetFieldWidth(50).SetText("coredump.bin")
	form.AddFormItem(input)
	form.AddButton("Download", func() {
		d.pages.RemovePage("coredump-download")
		savePath := strings.TrimSpace(input.GetText())
		if savePath == "" {
			d.log("[red]Coredump download aborted[-]: missing path")
			return
		}
		go d.performCoredumpDownload(savePath)
	})
	form.AddButton("Cancel", func() {
		d.pages.RemovePage("coredump-download")
	})
	form.SetBorder(true).SetTitle(fmt.Sprintf("Download Coredump (%d bytes)", d.coredumpSize))
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			d.pages.RemovePage("coredump-download")
			return nil
		}
		return event
	})

	d.pages.AddPage("coredump-download", centered(70, 7, form), true, true)
	d.app.SetFocus(form)
}

func (d *dashboard) performCoredumpDownload(path string) {
	d.log("Downloading coredump to %s...", path)

	var data []byte
	err := d.device.WithSession(func(s *msg.Session) error {
		var err error
		data, err = msg.ReadCoredump(s, 0, 0, 60*time.Second)
		return err
	})
	if err != nil {
		d.log("[red]Coredump download failed[-]: %v", err)
		return
	}

	// write to file
	err = os.WriteFile(path, data, 0644)
	if err != nil {
		d.log("[red]Coredump save failed[-]: %v", err)
		return
	}

	d.log("Coredump saved to %s (%d bytes)", path, len(data))
}

func (d *dashboard) deleteCoredump() {
	// check if coredump is available
	if d.coredumpSize == 0 {
		d.log("No coredump available")
		return
	}

	go func() {
		d.log("Deleting coredump...")
		err := d.device.WithSession(func(s *msg.Session) error {
			return msg.DeleteCoredump(s, 5*time.Second)
		})
		if err != nil {
			d.log("[red]Coredump delete failed[-]: %v", err)
			return
		}

		d.log("Coredump deleted")
		d.coredumpSize = 0
		d.queue(func() {
			d.updateInfo()
		})
	}()
}

func (d *dashboard) toggleLogStreaming() {
	if d.logDone != nil {
		d.stopLogStreaming()
	} else {
		d.startLogStreaming()
	}
}

func (d *dashboard) startLogStreaming() {
	// set state
	d.logDone = make(chan struct{})
	d.queue(func() {
		d.updateInfo()
	})

	// receive logs in background
	go d.loopLogReceive(d.logDone)
}

func (d *dashboard) loopLogReceive(logDone chan struct{}) {
	defer func() {
		d.logDone = nil
		d.queue(func() {
			d.updateInfo()
		})
	}()

	// create session
	session, err := d.device.NewSession()
	if err != nil {
		d.log("[red]Log streaming failed[-]: %v", err)
		return
	}

	// start log
	err = msg.StartLog(session, 5*time.Second)
	if err != nil {
		d.log("[red]Start log failed[-]: %v", err)
		_ = session.End(5 * time.Second)
		return
	}

	d.log("Log streaming started")

	// receive logs
	for {
		select {
		case <-logDone:
			_ = msg.StopLog(session, time.Second)
			_ = session.End(time.Second)
			d.log("Log streaming stopped")
			return
		case <-d.done:
			_ = msg.StopLog(session, time.Second)
			_ = session.End(time.Second)
			return
		default:
			// receive log message
			line, err := msg.ReceiveLog(session, time.Second)
			if err != nil {
				// ignore timeout errors
				continue
			}
			d.log("[blue]%s[-]", strings.TrimSpace(line))
		}
	}
}

func (d *dashboard) stopLogStreaming() {
	if d.logDone != nil {
		close(d.logDone)
	}
}
