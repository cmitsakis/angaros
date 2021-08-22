package main

import (
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"go.angaros.io/internal/license"
)

func tabAboutDeps() *container.TabItem {
	items := make([]*widget.AccordionItem, 0)
	for _, l := range license.Deps {
		items = append(items, widget.NewAccordionItem(l.Package, widget.NewLabel(l.License)))
	}
	content := container.NewVBox(
		widget.NewLabel("Angaros includes the following third-party libraries"),
		widget.NewAccordion(items...),
	)
	return container.NewTabItemWithIcon("Third-Party Licenses", theme.InfoIcon(), container.NewScroll(content))
}
