package main

import (
	"os/exec"
	"strings"
)

// Forecast is a representation of the current state of the weather.
type Forecast struct {
	raw        string
	time       TimeOfDay
	cloudiness Cloudiness
	raininess  Raininess
	visibility Visibility
	windiness  Windiness
}

// NewForecast parses the output of ~iajrz's climate program.
// TODO?: boolean randomize option to generate completely random weather as a demo mode?
func NewForecast() (Forecast, error) {
	out, err := exec.Command("/home/iajrz/climate").Output()
	if err != nil {
		return Forecast{}, err
	}
	rawWeather := string(out)
	return Forecast{
		raw:        rawWeather,
		time:       TimeOfDay(findSubstring(rawWeather, timeStrings)),
		cloudiness: Cloudiness(findSubstring(rawWeather, cloudStrings)),
		raininess:  Raininess(findSubstring(rawWeather, rainStrings)),
		visibility: Visibility(findSubstring(rawWeather, visibilityStrings)),
		windiness:  Windiness(findSubstring(rawWeather, windStrings)),
	}, nil
}

func (f Forecast) String() string {
	return f.raw
}

func findSubstring(s string, substrings []string) int {
	for i := range substrings {
		if strings.Contains(s, substrings[i]) {
			return i
		}
	}
	return 0
}

type TimeOfDay int

const (
	EarlyMorning TimeOfDay = iota
	Morning
	Afternoon
	Night
)

var timeStrings = []string{
	"early morning",
	"morning",
	"afternoon",
	"night",
}

type Cloudiness int

const (
	ClearSky Cloudiness = iota
	AlmostClear
	PartlyCloudy
	MostlyCloudy
	Cloudy
)

var cloudStrings = []string{
	"clear",
	"almost clear",
	"partly cloudy",
	"mostly cloudy",
	"cloudy",
}

type Raininess int

const (
	NoRain Raininess = iota
	Drizzle
	LightShower
	Shower
	HeavyShower
)

var rainStrings = []string{
	"no rain",
	"a drizzle",
	"a light shower",
	"a shower",
	"a heavy shower",
}

type Visibility int

const (
	NoFog Visibility = iota
	Haze
	Mist
	Fog
	HeavyFog
)

var visibilityStrings = []string{
	"visibility",
	"There's haze",
	"There's mist",
	"There's fog",
	"There's heavy fog",
}

type Windiness int

const (
	NoWind Windiness = iota
	Breeze
	StiffWind
)

var windStrings = []string{
	"no breeze",
	"light breeze",
	"stiff wind",
}
