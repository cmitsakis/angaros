package main

import (
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
)

func tabAbout() *container.TabItem {
	subTabs := container.NewAppTabs(tabAboutThis(), tabAboutDeps())
	return container.NewTabItemWithIcon("About", theme.InfoIcon(), container.NewMax(subTabs))
}
