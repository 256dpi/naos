package main

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
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

func showUnlockPrompt(app *tview.Application, pages *tview.Pages, device *device, onResult func(ok bool, err error)) {
	form := tview.NewForm()
	input := tview.NewInputField().SetLabel("Password").SetFieldWidth(30).SetMaskCharacter('*')
	form.AddFormItem(input)
	form.AddButton("Unlock", func() {
		pw := input.GetText()
		if pw == "" {
			return
		}
		go func() {
			ok, err := device.Unlock(pw)
			app.QueueUpdateDraw(func() {
				pages.RemovePage("unlock")
				onResult(ok, err)
			})
		}()
	})
	form.AddButton("Cancel", func() {
		pages.RemovePage("unlock")
		onResult(false, nil)
	})
	form.SetBorder(true).SetTitle(" Unlock Device ")
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			pages.RemovePage("unlock")
			onResult(false, nil)
			return nil
		}
		return event
	})
	pages.AddPage("unlock", centered(50, 7, form), true, true)
	app.SetFocus(form)
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
