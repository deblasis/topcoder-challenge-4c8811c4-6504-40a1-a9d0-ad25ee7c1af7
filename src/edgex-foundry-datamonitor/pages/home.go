package pages

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/deblasis/edgex-foundry-datamonitor/config"
	"github.com/deblasis/edgex-foundry-datamonitor/state"
	"github.com/edgexfoundry/go-mod-core-contracts/v2/dtos"
)

func parseURL(urlStr string) *url.URL {
	link, err := url.Parse(urlStr)
	if err != nil {
		fyne.LogError("Could not parse URL", err)
	}

	return link
}

func stateWithData(data binding.DataMap) *widget.Form {
	keys := data.Keys()
	items := make([]*widget.FormItem, len(keys))
	for i, k := range keys {
		data, err := data.GetItem(k)
		if err != nil {
			items[i] = widget.NewFormItem(k, widget.NewLabel(err.Error()))
		}
		items[i] = widget.NewFormItem(k, createItem(data))
	}

	return widget.NewForm(items...)
}

func createItem(v binding.DataItem) fyne.CanvasObject {
	switch val := v.(type) {
	case binding.Float:
		return widget.NewLabelWithData(binding.FloatToString(val))
	case binding.Int:
		return widget.NewLabelWithData(binding.IntToString(val))
	case binding.String:
		return widget.NewLabelWithData(val)
	default:
		return widget.NewLabel(fmt.Sprintf("%T", val))
	}
}

func homeScreen(w fyne.Window, appManager *state.AppManager) fyne.CanvasObject {
	// logo := canvas.NewImageFromResource(data.FyneScene)
	// logo.FillMode = canvas.ImageFillContain
	// if fyne.CurrentDevice().IsMobile() {
	// 	logo.SetMinSize(fyne.NewSize(171, 125))
	// } else {
	// 	logo.SetMinSize(fyne.NewSize(228, 167))
	// }
	a := fyne.CurrentApp()
	var contentContainer *fyne.Container

	redisHost, redisPort := appManager.GetRedisHostPort()
	connectionState := appManager.GetConnectionState()
	eventProcessor := appManager.GetEventProcessor()

	connectedContent := container.NewVBox()

	connectingContent := container.NewCenter(container.NewVBox(
		container.NewHBox(widget.NewProgressBarInfinite()),
	))

	disconnectedContent := container.NewCenter(container.NewVBox(
		widget.NewCard("You are currently disconnected from EdgeX Foundry",
			fmt.Sprintf("Would you like to connect to %v:%d?", redisHost, redisPort),
			container.NewCenter(
				widget.NewButtonWithIcon("Connect", theme.LoginIcon(), func() {
					if err := appManager.Connect(); err != nil {
						uerr := errors.New(fmt.Sprintf("Cannot connect\n%s", err))
						dialog.ShowError(uerr, w)
						//TODO: log this
					}
					appManager.Refresh()
				}),
			),
		),
	))

	eventsTable := renderEventsTable(eventProcessor.LastEvents.Get(), false)
	tableContainer := container.NewMax(eventsTable)

	totalNumberEventsBinding := binding.BindInt(config.Int(eventProcessor.TotalNumberEvents))
	totalNumberReadingsBinding := binding.BindInt(config.Int(eventProcessor.TotalNumberReadings))

	eventsPerSecondLastMinute := binding.BindFloat(config.Float(eventProcessor.EventsPerSecondLastMinute))
	readingsPerSecondLastMinute := binding.BindFloat(config.Float(eventProcessor.ReadingsPerSecondLastMinute))

	dashboardStats := container.NewGridWithRows(2,
		container.NewGridWithColumns(4,
			widget.NewLabelWithStyle("Total Number of Events", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithData(binding.IntToString(totalNumberEventsBinding)),
			widget.NewLabelWithStyle("Total Number of Readings", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithData(binding.IntToString(totalNumberReadingsBinding)),
		),
		container.NewGridWithColumns(4,
			widget.NewLabelWithStyle("Events per second", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithData(binding.FloatToString(eventsPerSecondLastMinute)),
			widget.NewLabelWithStyle("Readings per second", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithData(binding.FloatToString(readingsPerSecondLastMinute)),
		),
	)

	//form := stateWithData(boundState)

	//form.Hide()

	dashboardStats.Hide()
	tableContainer.Hide()

	switch connectionState {
	case state.Connected:
		contentContainer = connectedContent
		dashboardStats.Show()
		tableContainer = container.NewMax(eventsTable)
	case state.Connecting:
		contentContainer = connectingContent
		dashboardStats.Hide()
		tableContainer.Hide()
	case state.Disconnected:
		contentContainer = disconnectedContent
		dashboardStats.Hide()
		tableContainer = container.NewMax()
	}

	home := container.NewGridWithRows(2,
		container.NewVBox(
			contentContainer,
			dashboardStats,
		),
		tableContainer,
	)

	go func() {
		for {
			time.Sleep(100 * time.Millisecond)
			if appManager.GetConnectionState() != state.Connected {
				continue
			}
			//log.Printf("refreshing UI: events %v\n", ep.TotalNumberEvents)
			totalNumberEventsBinding.Set(eventProcessor.TotalNumberEvents)
			totalNumberReadingsBinding.Set(eventProcessor.TotalNumberReadings)
			eventsPerSecondLastMinute.Set(eventProcessor.EventsPerSecondLastMinute)
			readingsPerSecondLastMinute.Set(eventProcessor.ReadingsPerSecondLastMinute)
			eventsTable = renderEventsTable(eventProcessor.LastEvents.Get(), a.Preferences().BoolWithFallback(config.PrefEventsTableSortOrderAscending, config.DefaultEventsTableSortOrderAscending))

			//dashboardStats.Refresh()
			// home.Objects[0].(*fyne.Container).Objects[1] = table
			if len(tableContainer.Objects) > 0 {
				tableContainer.Objects[0] = eventsTable
			}
			//home.Refresh()

		}
	}()

	return home

}

func renderEventsTable(events []*dtos.Event, sortAsc bool) fyne.CanvasObject {

	// the slice is fifo, we reverse it so that the first element is the most recent
	evts := make([]*dtos.Event, len(events))
	copy(evts, events)

	if !sortAsc {
		for i, j := 0, len(evts)-1; i < j; i, j = i+1, j-1 {
			evts[i], evts[j] = evts[j], evts[i]
		}
	}

	renderCell := func(row, col int, label *widget.Label) {

		if len(evts) == 0 || row >= len(evts) {
			label.SetText("")
			return
		}

		event := evts[row]
		switch col {
		case 0:
			label.SetText(event.DeviceName)
		case 1:
			label.SetText(time.Unix(0, event.Origin).String())
		default:
			label.SetText("")
		}

	}

	t := widget.NewTable(
		func() (int, int) { return 6, 2 },
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(id widget.TableCellID, cell fyne.CanvasObject) {
			label := cell.(*widget.Label)
			switch id.Row {
			case 0:
				switch id.Col {
				// case 0:
				// 	label.SetText("ID")
				case 0:
					label.SetText("Device Name")
					label.TextStyle = fyne.TextStyle{Bold: true}
				case 1:
					label.SetText("Origin Timestamp")
					label.TextStyle = fyne.TextStyle{Bold: true}
				default:
					label.SetText("")
				}

			default:
				renderCell(id.Row-1, id.Col, label)
			}

		})
	// t.SetColumnWidth(0, 34)
	t.SetColumnWidth(0, 350)
	t.SetColumnWidth(1, 350)

	sortorder := "ascendingly"
	if !sortAsc {
		sortorder = "descendingly"
	}
	return container.NewBorder(
		container.NewVBox(layout.NewSpacer(), container.NewHBox(
			widget.NewLabelWithStyle("Last 5 events", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			layout.NewSpacer(),
			widget.NewLabelWithStyle(fmt.Sprintf("sorted %v by timestamp", sortorder), fyne.TextAlignTrailing, fyne.TextStyle{Italic: true}),
		)),
		nil,
		nil,
		nil,
		t,
	)

}