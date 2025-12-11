package main

import (
	"fmt"
	"time"

	"github.com/rivo/tview"
)

func centered(width, height int, primitive tview.Primitive) tview.Primitive {
	// create modal
	modal := tview.NewFlex().SetDirection(tview.FlexRow)
	modal.AddItem(tview.NewBox(), 0, 1, false)

	// create row
	row := tview.NewFlex().
		AddItem(tview.NewBox(), 0, 1, false).
		AddItem(primitive, width, 0, true).
		AddItem(tview.NewBox(), 0, 1, false)

	// add row to modal
	modal.AddItem(row, height, 0, true)
	modal.AddItem(tview.NewBox(), 0, 1, false)

	return modal
}

func showErrorModal(app *tview.Application, pages *tview.Pages, message string) {
	// create modal
	modal := tview.NewModal()
	modal.SetText(message)
	modal.AddButtons([]string{"OK"})
	modal.SetDoneFunc(func(_ int, _ string) {
		pages.RemovePage("error")
	})

	// show modal
	pages.AddPage("error", modal, true, true)
	app.SetFocus(modal)
}

func showProgressModal(app *tview.Application, pages *tview.Pages, text string) func() {
	// generate ID
	id := fmt.Sprintf("progress-%d", time.Now().UnixNano())

	// create modal
	modal := tview.NewModal().SetText(text)

	// show modal
	pages.AddPage(id, modal, true, true)
	app.SetFocus(modal)

	// return closer
	return func() {
		pages.RemovePage(id)
	}
}
