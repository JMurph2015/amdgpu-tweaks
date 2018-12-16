package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// FanSetpoint Describes a fan setpoint for interpolation
type FanSetpoint struct {
	Temp float64
	Fan  uint8
}

// Config Describes the configuration file format
type Config struct {
	PWMFile       string
	FanModeFile   string
	TempFile      string
	PollFrequency int32
	Hysteresis    float64
	Setpoint      []FanSetpoint
}

func interpolate(val float64, config Config) uint8 {
	sort.Slice(config.Setpoint, func(i, j int) bool {
		return config.Setpoint[i].Temp < config.Setpoint[j].Temp
	})

	for i := 0; i < len(config.Setpoint); i++ {
		if config.Setpoint[i].Temp < val {
			if i == len(config.Setpoint)-1 {
				return config.Setpoint[i].Fan
			}
			if config.Setpoint[i+1].Temp > val {
				aT := config.Setpoint[i].Temp
				aF := config.Setpoint[i].Fan
				bT := config.Setpoint[i+1].Temp
				bF := config.Setpoint[i+1].Fan
				return aF + uint8(math.Floor(float64(bF-aF)/(bT-aT)*(val-aT)))
			}
		} else {
			// This must be a temperature less than the first setpoint
			return config.Setpoint[i].Fan
		}
	}
	return 200
}

func check(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

func main() {
	confData, err := ioutil.ReadFile("/etc/amdgpu-tweaks/conf.toml")
	check(err)

	var config Config
	_, err = toml.Decode(string(confData), &config)
	check(err)

	err = ioutil.WriteFile(config.PWMFile, []byte("1"), 0644)
	check(err)

	prevTempBytes, err := ioutil.ReadFile(config.TempFile)
	check(err)
	prevTemp, err := strconv.Atoi(strings.TrimSuffix(string(prevTempBytes), "\n"))
	prevTemp = prevTemp / 1000
	check(err)
	for {
		currentTempBytes, err := ioutil.ReadFile(config.TempFile)
		check(err)
		currentTemp, err := strconv.Atoi(strings.TrimSuffix(string(currentTempBytes), "\n"))
		check(err)

		if math.Abs(float64(currentTemp-prevTemp)/1000) > config.Hysteresis {
			log.Printf("Need new PWM setting")
			newPWM := interpolate(float64(currentTemp)/1000, config)
			log.Printf("Writing new PWM value to the card: %d", newPWM)
			err = ioutil.WriteFile(config.PWMFile, []byte(fmt.Sprintf("%d", newPWM)), 0644)
			check(err)
		}

		prevTemp = currentTemp
		time.Sleep(time.Duration(config.PollFrequency) * time.Millisecond)
	}
}
