package form

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	widgetx "fyne.io/x/fyne/widget"
)

func showModal(w fyne.Window, title, confirm, dismiss string, content fyne.CanvasObject, callback func() error) {
	var modal *widget.PopUp
	contentScroll := container.NewVScroll(content)
	top := container.NewVBox(
		widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
	)
	bottom := container.NewHBox(
		layout.NewSpacer(),
		widget.NewButtonWithIcon(dismiss, theme.CancelIcon(), func() {
			modal.Hide()
		}),
		widget.NewButtonWithIcon(confirm, theme.ConfirmIcon(), func() {
			err := callback()
			if err != nil {
				dialog.ShowError(err, w)
			} else {
				modal.Hide()
			}
		}),
		layout.NewSpacer(),
	)
	contentBox := container.NewBorder(top, bottom, nil, nil, contentScroll)
	modal = widget.NewModalPopUp(contentBox, w.Canvas())
	// calculate the size of the modal by creating another temporary modal without scroll and getting it's size
	mSize := func() fyne.Size {
		tempContentBox := container.NewBorder(top, bottom, nil, nil, content)
		tempModal := widget.NewModalPopUp(tempContentBox, w.Canvas())
		s := tempContentBox.Size()
		tempModal.Hide()
		return fyne.Size{Height: s.Height + 10, Width: s.Width}
	}()
	mSize = mSize.Min(w.Canvas().Size())
	modal.Resize(mSize)
	modal.Show()
}

func NewValue(w fyne.Window, description string, onEdit func(chan<- string), onClear func(chan<- string)) *fyne.Container {
	label := widget.NewLabel("")
	labelUpdates := make(chan string, 1)
	go func() {
		for s := range labelUpdates {
			label.SetText(s)
		}
	}()
	return container.NewHBox(
		label,
		widget.NewButton("Edit", func() { onEdit(labelUpdates) }),
		widget.NewButton("Clear", func() {
			content := widget.NewLabel("Are you sure you want to clear this value?")
			showModal(w, "Clear Value", "Confirm", "Cancel", content, func() error {
				onClear(labelUpdates)
				return nil
			})
		}),
		widget.NewLabelWithStyle(description, fyne.TextAlignLeading, fyne.TextStyle{Italic: true}),
	)
}

func ShowEntryPopup(w fyne.Window, title, description, placeHolder, existingValue string, onSubmit func(string) error) {
	entry := widget.NewEntry()
	entry.SetText(existingValue)
	entry.SetPlaceHolder(placeHolder)
	content := container.NewVBox(
		widget.NewLabel(description),
		entry,
	)
	showModal(w, title, "Save", "Cancel", content, func() error {
		entryText := entry.Text
		return onSubmit(entryText)
	})
}

func ShowEntryCompletionPopup(w fyne.Window, title, description, placeHolder, existingValue string, options []string, filter func([]string, string) []string, onSubmit func(string) error) {
	entry := widgetx.NewCompletionEntry([]string{})
	entry.OnChanged = func(s string) {
		if len(s) < 3 {
			entry.HideCompletion()
			return
		}
		matches := filter(options, entry.Text)
		if len(matches) == 0 {
			entry.HideCompletion()
			return
		}
		entry.SetOptions(matches)
		entry.ShowCompletion()
	}
	entry.SetText(existingValue)
	entry.SetPlaceHolder(placeHolder)
	content := container.NewVBox(
		widget.NewLabel(description),
		entry,
	)
	showModal(w, title, "Save", "Cancel", content, func() error {
		entryText := entry.Text
		return onSubmit(entryText)
	})
}

func ShowSelectionPopup(w fyne.Window, title, description, action string, options []string, existingSelection string, onSubmit func(string, int) error) {
	selectWidget := widget.NewSelect(options, nil)
	for i, option := range options {
		if option == existingSelection {
			selectWidget.SetSelectedIndex(i)
			break
		}
	}
	content := container.NewVBox(
		widget.NewLabel(description),
		selectWidget,
	)
	showModal(w, title, action, "Cancel", content, func() error {
		if selectWidget.SelectedIndex() >= 0 && selectWidget.SelectedIndex() < len(options) {
			return onSubmit(options[selectWidget.SelectedIndex()], selectWidget.SelectedIndex())
		} else {
			return onSubmit("", -1)
		}
	})
}

func FilterOptions(options []string, input string) []string {
	if len(input) < 3 {
		return nil
	}
	inputLower := strings.ToLower(input)
	filteredOptions := make([]string, 0)
	for _, option := range options {
		if strings.Contains(strings.ToLower(option), inputLower) {
			filteredOptions = append(filteredOptions, option)
		}
	}
	return filteredOptions
}

type FormFieldType int

const (
	FormFieldTypeEntry = FormFieldType(iota)
	FormFieldTypeRadio
	FormFieldTypeDropdown
)

type FormField struct {
	Name          string
	Type          FormFieldType
	ExistingValue string
	Options       []string
	OptionsValues []string
	PlaceHolder   string
	Description   string
	ReadOnly      bool
}

func ShowFormPopup(w fyne.Window, title, description string, fields []FormField, onSubmit func([]string) error) {
	f := &widget.Form{}
	formWidgets := make([]fyne.CanvasObject, 0, len(fields))
	for _, field := range fields {
		var fieldWidget fyne.CanvasObject
		switch field.Type {
		case FormFieldTypeEntry:
			fieldWidgetEntry := widget.NewEntry()
			if field.ExistingValue != "" {
				fieldWidgetEntry.SetText(field.ExistingValue)
			}
			fieldWidgetEntry.SetPlaceHolder(field.PlaceHolder)
			if field.ReadOnly {
				fieldWidgetEntry.Disable()
			}
			fieldWidget = fieldWidgetEntry
		case FormFieldTypeRadio:
			fieldWidgetRadio := widget.NewRadioGroup(field.Options, nil)
			if field.ExistingValue != "" {
				if field.OptionsValues != nil {
					for i, v := range field.OptionsValues {
						if field.ExistingValue == v {
							fieldWidgetRadio.SetSelected(field.Options[i])
							break
						}
					}
				} else {
					fieldWidgetRadio.SetSelected(field.ExistingValue)
				}
			}
			fieldWidget = fieldWidgetRadio
		}
		formWidgets = append(formWidgets, fieldWidget)
		f.Append(field.Name+":", fieldWidget)
		if field.Description != "" {
			l := widget.NewLabel(field.Description)
			// l.Wrapping = fyne.TextWrapWord
			f.Append("", l)
		}
	}
	content := container.NewVBox(
		widget.NewLabel(description),
		f,
	)
	showModal(w, title, "Save", "Cancel", content, func() error {
		submittedValues := make([]string, 0, len(fields))
		for i, field := range fields {
			switch field.Type {
			case FormFieldTypeEntry:
				submittedValues = append(submittedValues, formWidgets[i].(*widget.Entry).Text)
			case FormFieldTypeRadio:
				submittedValues = append(submittedValues, formWidgets[i].(*widget.RadioGroup).Selected)
			default:
				submittedValues = append(submittedValues, "")
			}
		}
		return onSubmit(submittedValues)
	})
}

func ShowCustomPopup(w fyne.Window, title, description, confirmText, cancelText string, content fyne.CanvasObject, onSubmit func() error) {
	showModal(w, title, confirmText, cancelText, content, onSubmit)
}
