package main

import (
	"fmt"
	"math"

	"github.com/brutella/hc/accessory"
	hcAccessory "github.com/brutella/hc/accessory"
	"github.com/brutella/hc/characteristic"
	"github.com/brutella/hc/log"
	"github.com/brutella/hc/service"
	"github.com/nidi-to/go-frigidaire"
	"github.com/nidi-to/go-frigidaire/attributes"
)

// AC adds homekit friendly methods to frigidaire.Appliance
type AC struct {
	*frigidaire.Appliance
}

// CoolingMode returns the cooling mode
func (ac *AC) CoolingMode() int {
	mode := ac.Appliance.Get(attributes.CoolingMode)
	if mode == nil {
		return 0
	}

	state := mode.Int()
	switch state {
	case int(attributes.CoolingModeOff):
		return 0
	case int(attributes.CoolingModeFan):
		return 1
	case int(attributes.CoolingModeEcon):
		return 2
	case int(attributes.CoolingModeCool):
		return 3
	}

	return 0
}

// CurrentTemperature returns the current temperature in celsius
func (ac *AC) CurrentTemperature() float64 {
	temp := ac.Appliance.Get(attributes.TemperatureCurrent)
	if temp != nil {
		return fahrenheitToCelcius(temp.Int())
	}

	return 0.0
}

// TargetTemperature returns the target temperature in celsius
func (ac *AC) TargetTemperature() float64 {
	temp := ac.Appliance.Get(attributes.TemperatureTarget)
	if temp != nil {
		return fahrenheitToCelcius(temp.Int())
	}

	return 0.0
}

// RotationSpeed returns the fan speed
func (ac *AC) RotationSpeed() float64 {
	mode := ac.Appliance.Get(attributes.FanSpeed)
	if mode == nil {
		return 0
	}

	state := mode.Int()
	switch state {
	case int(attributes.FanSpeedAuto):
		return 1
	case int(attributes.FanSpeedLow):
		return 25
	case int(attributes.FanSpeedMed):
		return 50
	case int(attributes.FanSpeedHigh):
		return 100
	}

	return 0
}

// Status notes if the appliance is on or off
func (ac *AC) Status() int {
	mode := ac.Appliance.Get(attributes.CoolingMode)
	if mode == nil {
		return 0
	}

	state := mode.Int()
	if state == int(attributes.CoolingModeOff) {
		return 0
	}

	return 1
}

// NewAC creates a new AC
func NewAC(appliance *frigidaire.Appliance, prefix string) *accessory.Accessory {
	ac := &AC{Appliance: appliance}
	name := fmt.Sprintf("%s-%s", prefix, appliance.Label)
	log.Debug.Printf("Adding appliance %s (SN: %s)\n", name, appliance.SerialNumber)
	info := hcAccessory.Info{
		Name:             name,
		SerialNumber:     appliance.SerialNumber,
		Manufacturer:     appliance.Manufacturer,
		Model:            appliance.Model,
		FirmwareRevision: appliance.NIUVersion,
		// ID:               uint64(appliance.ID),
	}

	var refresh func() = func() {}
	acc := accessory.New(info, accessory.TypeAirConditioner)
	cooler := service.NewHeaterCooler()

	// State
	coolingMode := ac.CoolingMode()
	log.Debug.Printf("cooling mode: %d\n", coolingMode)
	cooler.CurrentHeaterCoolerState.SetValue(coolingMode)
	cooler.CurrentHeaterCoolerState.SetMaxValue(3)
	cooler.CurrentHeaterCoolerState.SetMinValue(0)
	cooler.CurrentHeaterCoolerState.SetStepValue(1)
	cooler.CurrentHeaterCoolerState.OnValueRemoteGet(func() int {
		log.Debug.Println("state:Get")
		go refresh()
		return ac.CoolingMode()
	})

	cooler.TargetHeaterCoolerState.SetValue(coolingMode)
	cooler.TargetHeaterCoolerState.SetMaxValue(4)
	cooler.TargetHeaterCoolerState.SetMinValue(0)
	cooler.TargetHeaterCoolerState.SetStepValue(1)
	cooler.TargetHeaterCoolerState.OnValueRemoteUpdate(func(targetState int) {
		newState := attributes.CoolingModeOff

		switch targetState {
		case characteristic.CurrentHeaterCoolerStateInactive:
			newState = attributes.CoolingModeEcon
		case characteristic.CurrentHeaterCoolerStateIdle:
			newState = attributes.CoolingModeFan
		case characteristic.CurrentHeaterCoolerStateHeating:
			newState = attributes.CoolingModeCool
		}
		log.Debug.Printf("state:set %d => %d\n", targetState, newState)

		err := ac.Appliance.Update(attributes.CoolingMode, int(newState))
		if err != nil {
			cooler.CurrentHeaterCoolerState.SetValue(targetState)
		}
	})

	// Current Temp
	currentTemperature := ac.CurrentTemperature()
	log.Debug.Printf("current temp: %f\n", currentTemperature)
	cooler.CurrentTemperature.SetValue(currentTemperature)
	cooler.CurrentTemperature.OnValueRemoteGet(func() float64 {
		go refresh()
		temp := ac.CurrentTemperature()
		log.Debug.Printf("current-temp:get %f\n", temp)
		return temp
	})

	// Active
	activeStatus := ac.Status()
	log.Debug.Printf("active: %d\n", activeStatus)
	cooler.Active.SetValue(activeStatus)
	cooler.Active.OnValueRemoteGet(func() int {
		go refresh()
		mode := ac.Appliance.Get(attributes.CoolingState)
		log.Debug.Printf("active:get %v\n", mode)
		if mode == nil {
			return 0
		}

		state := mode.Int()
		if state == int(attributes.CoolingModeOff) {
			return 0
		}

		return 1
	})

	cooler.Active.OnValueRemoteUpdate(func(desiredState int) {
		log.Debug.Printf("active:update %d\n", desiredState)
		err := ac.Appliance.Update(attributes.CoolingMode, desiredState)
		if err != nil {
			cooler.Active.SetValue(desiredState)
		}
	})

	// Rotation Speed
	speed := ac.RotationSpeed()
	rotationSpeed := characteristic.NewRotationSpeed()
	log.Debug.Printf("speed: %f\n", speed)
	rotationSpeed.SetValue(speed)
	rotationSpeed.SetMinValue(0)
	rotationSpeed.SetMaxValue(100)
	rotationSpeed.SetStepValue(25)
	rotationSpeed.OnValueRemoteGet(func() float64 {
		go refresh()
		speed := ac.RotationSpeed()
		log.Debug.Printf("speed:get %f\n", speed)
		return speed
	})

	rotationSpeed.OnValueRemoteUpdate(func(targetSpeed float64) {
		var desiredMode attributes.FanSpeeds
		switch {
		case targetSpeed < 25:
			desiredMode = attributes.FanSpeedAuto
		case targetSpeed <= 25:
			desiredMode = attributes.FanSpeedLow
		case targetSpeed > 25 && targetSpeed <= 50:
			desiredMode = attributes.FanSpeedMed
		case targetSpeed >= 75:
			desiredMode = attributes.FanSpeedHigh
		}

		log.Debug.Printf("speed:set %f, %d\n", targetSpeed, desiredMode)

		err := ac.Appliance.Update(attributes.FanSpeed, int(desiredMode))
		if err != nil {
			rotationSpeed.SetValue(targetSpeed)
		}

	})
	cooler.Service.AddCharacteristic(rotationSpeed.Characteristic)

	// Target Temp
	targetTemperature := ac.TargetTemperature()
	log.Debug.Printf("target-temp: %f\n", targetTemperature)
	coolingTreshold := characteristic.NewCoolingThresholdTemperature()
	coolingTreshold.SetValue(targetTemperature)
	coolingTreshold.SetMinValue(20)
	coolingTreshold.SetMaxValue(30)
	coolingTreshold.SetStepValue(0.5)
	coolingTreshold.OnValueRemoteGet(func() float64 {
		go refresh()
		return ac.TargetTemperature()
	})
	coolingTreshold.OnValueRemoteUpdate(func(targetTemperature float64) {
		fahrenheit := math.Round((targetTemperature * 9.0 / 5.0) + 32)
		err := ac.Appliance.Update(attributes.TemperatureTarget, int(fahrenheit*10))
		if err != nil {
			coolingTreshold.SetValue(targetTemperature)
		}
	})
	cooler.Service.AddCharacteristic(coolingTreshold.Characteristic)

	acc.AddService(cooler.Service)

	ac.OnUpdatedTelemetry(func() {
		coolingMode := ac.CoolingMode()
		cooler.CurrentHeaterCoolerState.SetValue(coolingMode)
		cooler.TargetHeaterCoolerState.SetValue(coolingMode)
		cooler.CurrentTemperature.SetValue(ac.CurrentTemperature())
		cooler.Active.SetValue(ac.Status())
		coolingTreshold.SetValue(ac.TargetTemperature())
		rotationSpeed.SetValue(ac.RotationSpeed())
	})

	return acc
}
