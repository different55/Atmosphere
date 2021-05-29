package main

import (
	"bufio"
	"fmt"
	"github.com/muesli/termenv"
	"github.com/ojrac/opensimplex-go"
	"io"
	"os"
	"path"
	"unicode"
)

// Style represents a pair of colors, foreground and background.
type Style struct {
	fg termenv.Color
	bg termenv.Color
}

// Convert is a wrapper for termenv.Profile.Convert, it converts the foreground
// and background colors of a Style to be within a given terminal's Profile.
func (s Style) Convert(profile termenv.Profile) Style {
	return Style{
		profile.Convert(s.fg),
		profile.Convert(s.bg),
	}
}

// String fills the Stringer interface and returns an ANSI escape code
// formatted to produce text in the specified Style.
func (s Style) String() string {
	if s.fg == nil {
		s.fg = termenv.NoColor{}
	}
	if s.bg == nil {
		s.bg = termenv.NoColor{}
	}
	// TODO: Looks like the background color is overriding the foreground color. What's the proper way to do this?
	return fmt.Sprintf("%s%s;%sm", termenv.CSI, s.fg.Sequence(false), s.bg.Sequence(true))
}

// Scene holds all the necessary information to make a dynamic, weather-y ASCII
// landscape.
type Scene struct {
	foreground [][]rune
	background [][]rune
	windground [][]rune
	depth      [][]rune
	forecast   Forecast

	generator opensimplex.Noise

	Width  int
	Height int
}

const (
	fgPath    = "foreground.txt"
	depthPath = "depth.txt"
	windPath  = "wind.txt"

	earlyPath     = "early.txt"
	morningPath   = "morning.txt"
	afternoonPath = "afternoon.txt"
	nightPath     = "night.txt"
)

var timeToPath = []string{
	earlyPath,
	morningPath,
	afternoonPath,
	nightPath,
}

// NewScene accepts a path to a folder containing foreground, windground,
// depth, and background files and loads them from disk as needed to generate
// imagery for the given forecast.
func NewScene(scenePath string, forecast Forecast) (Scene, error) {
	var s Scene
	var err error

	s.forecast = forecast
	s.generator = opensimplex.NewNormalized(0)

	s.foreground, err = readRunesFromFile(path.Join(scenePath, fgPath))
	if err != nil {
		return s, err
	}
	s.depth, err = readRunesFromFile(path.Join(scenePath, depthPath))
	if err != nil {
		return s, err
	}

	s.windground, err = readRunesFromFile(path.Join(scenePath, windPath))
	if err != nil {
		return s, err
	}

	bgPath := timeToPath[forecast.time]
	s.background, err = readRunesFromFile(path.Join(scenePath, bgPath))
	if err != nil {
		return s, err
	}

	s.normalize()

	/*m.depthData = make([][]uint8, 0)
	for i := range depthRunes {
		m.depthData = append(m.depthData, make([]uint8, 0))
		for j := range depthRunes[i] {
			m.depthData[i] = append(m.depthData[i], uint8(depthRunes[i][j])-48)
		}
	}*/
	return s, nil
}

// normalize adjusts the foreground, windground, depth map, and background to
// have matching widths and heights. It adjusts by cropping to the shortest
// number of lines between the four and adjusts each line to the shortest of
// any lines in any of the four maps.
func (s *Scene) normalize() {
	scenes := [...][][]rune{s.foreground, s.windground, s.depth, s.background}

	s.Height = 999999999
	for i := range scenes {
		if len(scenes[i]) < s.Height {
			s.Height = len(scenes[i])
		}
	}

	for i := range scenes {
		scenes[i] = scenes[i][:s.Height]
	}

	s.Width = 999999999
	for j := 0; j < s.Height; j++ {
		for i := range scenes {
			if len(scenes[i][j]) < s.Width {
				s.Width = len(scenes[i][j])
			}
		}

		for i := range scenes {
			scenes[i][j] = scenes[i][j][:s.Width]
		}
	}

	// TODO: Change this to a map? Map can be iterated over and still referenced by name.
	s.foreground = scenes[0]
	s.windground = scenes[1]
	s.depth = scenes[2]
	s.background = scenes[3]
}

func (s Scene) GetCell(x, y, time int) (rune, Style) {
	fx := float64(x)
	fy := float64(y)
	ftime := float64(time)
	fcloud := float64(s.forecast.cloudiness)
	fwind := float64(s.forecast.windiness)
	frain := float64(s.forecast.raininess)

	char := ' '
	style := Style{
		termenv.ANSI256Color(255),
		termenv.ANSI256Color(232),
	}

	// Out of bounds
	if y >= s.Height || y < 0 {
		return char, Style{}
	}
	if x >= s.Width || x < 0 {
		return char, Style{}
	}

	depth := uint8(s.depth[y][x] - 48)

	// Char selection
	char = s.foreground[y][x]
	// Pull from wind map if the current cell is windswept.
	if fwind > 0.0 && s.generator.Eval3(fx/40+fwind/8*ftime*2, fy/20, ftime/5+fwind/10*ftime/2) > 0.6 {
		char = s.windground[y][x]
	}
	// Depth 9 is considered "transparent," so use the char from the background.
	if depth == 9 {
		char = s.background[y][x]
	}
	if frain > 0.0 && 0.1+frain/20 > s.generator.Eval3(fx+ftime*fwind*3, fy-ftime*5, ftime/15) {
		char = '|'
		if fwind > 1.0 {
			char = '/'
		}
	}

	// Style selection
	// Calculate fog
	fog := (depth - 1) + uint8(s.forecast.visibility-1)*2
	if s.forecast.visibility == 0 {
		fog = (depth + uint8(s.forecast.visibility)) / 2
	}
	// Don't show fog in the sky during the night.
	if depth == 9 && (s.forecast.time == Night || s.forecast.time == EarlyMorning) {
		fog = 0
	}

	// Add clouds
	cloudiness := 0
	if depth > 8 && fcloud/5 > s.generator.Eval3(fx/50+ftime/24+ftime*(fwind/8), fy/12, 1000+ftime/80) {
		cloudiness += 10
	}
	if depth > 6 && fcloud/7 > s.generator.Eval3(fx/20+ftime/14+ftime*(fwind/8), fy/6, 0+ftime/80) {
		cloudiness += 10
	}

	// At night, fog obscures distant objects with darkness.
	if s.forecast.time == Night || s.forecast.time == EarlyMorning {
		style.fg = termenv.ANSI256Color(255 - fog - fog/2)
	} else {
		// Merge fog with clouds during daytime.
		cloudiness += int(fog)
	}

	if cloudiness > 23 {
		cloudiness = 23
	}

	if cloudiness > 0 {
		style.bg = termenv.ANSI256Color(232 + cloudiness)
	} else {
		style.bg = termenv.ANSI256Color(232 + fog)
	}
	return char, style
}

// readRunesFromFile reads the file at the given path and reads it character by
// character, line by line into a slice of slices of runes.
func readRunesFromFile(filepath string) ([][]rune, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}

	br := bufio.NewReader(file)
	img := make([][]rune, 0)
	img = append(img, make([]rune, 0))

	for {
		r, s, err := br.ReadRune()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		// Invalid unicode characters are skipped.
		if r == unicode.ReplacementChar && s == 1 {
			continue
		}

		// Start a new slice on newline, otherwise append character to current slice.
		if r == '\n' {
			img = append(img, make([]rune, 0))
		} else {
			img[len(img)-1] = append(img[len(img)-1], r)
		}
	}

	return img, nil
}
