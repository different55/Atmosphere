package main

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/reflow/wordwrap"
	"github.com/muesli/termenv"
	"strings"
	"time"
)

func main() {
	app := tea.NewProgram(model{startTime: time.Now()})
	if err := app.Start(); err != nil {
		panic(err)
	}
}

// Queues the initial loading of the forecast and
func (m model) Init() tea.Cmd {
	return tea.Batch(updateForecast, renderOften)
}

var updateForecast = func() tea.Msg {
	forecast, err := NewForecast()
	if err != nil {
		return err
	}
	return forecast
}

var updateForecastOften = tea.Tick(20*time.Second, func(t time.Time) tea.Msg {
	return updateForecast()
})

var makeSceneMsg = func(f Forecast) func() tea.Msg {
	return func() tea.Msg {
		scene, err := NewScene("scene1", f)
		if err != nil {
			return err
		}
		return scene
	}
}

type fadeOut float32
type fadeIn float32

// Used to update the progress of a fade out transition.
var tickFadeOut = tea.Every(time.Millisecond*100, func(t time.Time) tea.Msg {
	return fadeOut(0.05)
})

// Used to update the progress of a fade in transition.
var tickFadeIn = tea.Every(time.Millisecond*100, func(t time.Time) tea.Msg {
	return fadeIn(0.05)
})

type renderTick struct{}

// Used to update the screen at least once per second.
var renderOften = tea.Tick(time.Second, func(t time.Time) tea.Msg {
	return renderTick{}
})

// Update updates the internal state of the model and queues any events that
// need to be scheduled.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := []tea.Cmd{}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "ctrl+d", "q":
			return m, tea.Quit
		case "esc":
			// TODO: Menu for options. Maybe 100% randomized weather?
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case Forecast:
		// Re-queue the forecast polling.
		cmds = append(cmds, updateForecastOften)

		// No action required if the forecast hasn't changed.
		if msg == m.Forecast {
			break
		}

		// If this isn't the initial startup forecast, we need to fade out the scene.
		if m.Forecast != (Forecast{}) {
			m.fading = true
			cmds = append(cmds, tickFadeOut)
		}

		// In all cases, we need to load a scene with the forecast.
		m.Forecast = msg
		cmds = append(cmds, makeSceneMsg(m.Forecast))

	case Scene:
		if m.Scene.foreground == nil {
			m.Scene = msg
		} else {
			m.nextScene = msg
		}

	case fadeOut:
		m.fadeProgress += float32(msg)
		if m.fadeProgress >= 1.0 {
			m.fadeProgress = 1.0
			// To unfade, we need a Scene ready to rock. If we don't have one, stall.
			if m.nextScene.foreground == nil {
				cmds = append(cmds, tickFadeOut)
				break
			}

			m.Scene = m.nextScene
			m.nextScene = Scene{}
			cmds = append(cmds, tickFadeIn)
			break
		}
		cmds = append(cmds, tickFadeOut)

	case fadeIn:
		m.fadeProgress -= float32(msg)
		if m.fadeProgress <= 0.0 {
			m.fadeProgress = 0.0
			m.fading = false
			break
		}
		cmds = append(cmds, tickFadeIn)

	case error:
		m.err = msg

	case renderTick:
		cmds = append(cmds, renderOften)
	}

	return m, tea.Batch(cmds...)
}

// View lays out the screen and queries the Scene for its contents.
// It also handles transitions when changing from Scene to Scene.
func (m model) View() string {
	if m.err != nil {
		return m.err.Error()
	}

	weather := wordwrap.String(m.Forecast.String(), m.width)
	m.height -= strings.Count(weather, "\n")
	weather = strings.TrimSpace(weather) // Trim trailing newline

	output := ""
	lastStyle := Style{}
	profile := termenv.EnvColorProfile()

	// Center scene in terminal
	xOff := (m.Scene.Width - m.width) / 2
	yOff := (m.Scene.Height - m.height) / 2

	for y := yOff; y < yOff+m.height; y++ {
		for x := xOff; x < xOff+m.width; x++ {
			char, style := m.Scene.GetCell(x, y, int(time.Since(m.startTime).Seconds()))
			style = style.Convert(profile)

			// For wipe transitions, replace character with blank space.
			if float32((x-xOff)/2+y-yOff) < m.fadeProgress*float32(m.width/2+m.height) {
				char = ' '
				style = Style{}
			}

			// Only output formatting escape codes if the style's changed since the last cell.
			if lastStyle.String() != style.String() {
				output += style.String()
				lastStyle = style
			}

			output += string(char)
		}
		output += "\n"
	}

	output += termenv.CSI + termenv.ResetSeq + "m"
	output += weather

	return output
}

type model struct {
	Forecast
	Scene
	err          error
	nextScene    Scene
	fading       bool
	fadeProgress float32
	width        int
	height       int
	startTime    time.Time
}
