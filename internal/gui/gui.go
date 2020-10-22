package gui

import (
	"regexp"
	"sync"

	"fyne.io/fyne/layout"

	"fyne.io/fyne"
	"fyne.io/fyne/app"
	"fyne.io/fyne/widget"
	"github.com/galamiram/nadctl/internal/nadapi"
	log "github.com/sirupsen/logrus"
)

// GUI - nadctl GUI object
type GUI struct {
	app     fyne.App
	window  fyne.Window
	device  *nadapi.Device
	buttons map[string]*widget.Button
	labels  map[string]*settingLabel
}

type refreshFuncType = func() (string, error)

type settingLabel struct {
	label       *widget.Label
	refreshFunc refreshFuncType
	mux         sync.Mutex
}

func (s *settingLabel) refresh() error {
	data, err := s.refreshFunc()
	if err != nil {
		return err
	}
	//s.mux.Lock()
	s.label.Text = data
	s.label.Refresh()
	//s.mux.Unlock()
	return nil
}

// New - create new nadctl GUI
func New(device *nadapi.Device) (*GUI, error) {
	var (
		gui *GUI
	)

	gui = &GUI{
		device:  device,
		app:     app.New(),
		buttons: make(map[string]*widget.Button),
		labels:  make(map[string]*settingLabel),
	}

	model, err := gui.device.GetModel()
	if err != nil {
		return nil, err
	}

	gui.window = gui.app.NewWindow(model)
	gui.window.Resize(fyne.NewSize(300, 400))
	gui.window.SetContent(
		fyne.NewContainerWithLayout(
			layout.NewGridLayoutWithRows(4),
			fyne.NewContainerWithLayout(
				layout.NewGridLayoutWithColumns(2),
				fyne.NewContainerWithLayout(
					layout.NewGridLayoutWithRows(2),
					gui.addLabel("Power", gui.device.GetPowerState),
					gui.addButton("Power", func() {
						gui.device.PowerToggle()
					}),
				),
				fyne.NewContainerWithLayout(
					layout.NewGridLayoutWithRows(2),
					gui.addLabel("Mute", gui.device.GetMuteStatus),
					gui.addButton("Mute", func() {
						gui.device.ToggleMute()
					}),
				),
			),
			fyne.NewContainerWithLayout(
				layout.NewGridLayoutWithColumns(3),
				gui.addButton("Vol -", func() {
					go gui.device.TuneVolume(nadapi.DirectionDown)
				}),
				fyne.NewContainerWithLayout(
					layout.NewCenterLayout(),
					gui.addLabel("Volume", gui.device.GetVolume),
				),
				gui.addButton("Vol +", func() {
					go gui.device.TuneVolume(nadapi.DirectionUp)
				}),
			),
			fyne.NewContainerWithLayout(
				layout.NewGridLayoutWithColumns(3),
				gui.addButton("Brgtns -", func() {
					go gui.device.ToggleBrightness(nadapi.DirectionDown)
				}),
				fyne.NewContainerWithLayout(
					layout.NewCenterLayout(),
					gui.addLabel("Brightness", gui.device.GetBrightness),
				),
				gui.addButton("Brgtns +", func() {
					go gui.device.ToggleBrightness(nadapi.DirectionUp)
				}),
			),
			fyne.NewContainerWithLayout(
				layout.NewGridLayoutWithColumns(3),
				gui.addButton("<", func() {
					go gui.device.ToggleSource(nadapi.DirectionDown)
				}),
				fyne.NewContainerWithLayout(
					layout.NewCenterLayout(),
					gui.addLabel("Source", gui.device.GetSource),
				),
				gui.addButton(">", func() {
					go gui.device.ToggleSource(nadapi.DirectionUp)
				}),
			),
		),
	)
	gui.refreshLabels()
	go gui.listener()
	return gui, nil
}

// Start - start the GUI
func (gui *GUI) Start() {
	gui.window.Show()
	gui.app.Run()
}

func (gui *GUI) addButton(text string, action func()) *widget.Button {
	button := widget.NewButton(text, func() { go action() })
	gui.buttons[text] = button
	return button
}

func (gui *GUI) addLabel(setting string, f refreshFuncType) *widget.Label {
	label := &settingLabel{
		label:       widget.NewLabelWithStyle("", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		refreshFunc: f,
	}
	label.label.Alignment = fyne.TextAlignCenter
	gui.labels[setting] = label
	return label.label
}

func (gui *GUI) refreshLabels() {
	for _, label := range gui.labels {
		label.refresh()
	}
}

func (gui *GUI) listener() {
	r, _ := gui.device.GetRead()
	for {
		str, err := r.ReadString('\n')
		if err != nil {
			return
		}

		f := getFunctionName(str)
		log.WithField("f", f).Debug()
		if lbl, ok := gui.labels[f[1]]; ok {
			lbl.label.Text = f[2]
			lbl.label.Refresh()
		}
	}
}

func getFunctionName(s string) []string {
	compRegEx := regexp.MustCompile(`.*\.([a-zA-Z]*)=(.*)\r\n`)
	match := compRegEx.FindStringSubmatch(s)
	if len(match) > 0 {
		return match
	}
	return []string{}
}
