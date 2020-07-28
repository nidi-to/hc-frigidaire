package main

import (
	"os"
	"time"

	"github.com/brutella/hc"
	hcAccessory "github.com/brutella/hc/accessory"
	"github.com/brutella/hc/log"
	service "github.com/nidi-to/go-frigidaire"
)

// version gets overridden during build
var version = "0.0.0"

func main() {
	if os.Getenv("DEBUG") != "" {
		log.Debug.Enable()
	}

	username := os.Getenv("FRIGIDAIRE_USERNAME")
	password := os.Getenv("FRIGIDAIRE_PASSWORD")
	pin := os.Getenv("HOMEKIT_PIN")
	if pin == "" {
		pin = "12344321"
	}

	prefix := os.Getenv("APPLIANCE_PREFIX")
	if prefix == "" {
		pin = "AC"
	}

	client, err := service.NewSession(username, password)

	if err != nil {
		log.Info.Fatalf("Could not create service: %v", err)
		os.Exit(2)
	}

	var accessories = make([]*hcAccessory.Accessory, 0)

	err = client.RefreshTelemetry()
	if err != nil {
		log.Info.Fatal(err)
	}

	for _, device := range client.Appliances {
		log.Debug.Printf("Found %s\n", device.Label)
		acc := NewAC(device, prefix)
		accessories = append(accessories, acc)
	}

	log.Info.Printf("Initializing with %d accessories and pin: %s\n", len(accessories), pin)

	// configure the ip transport
	config := hc.Config{Pin: pin}
	refreshTicker := time.NewTicker(1 * time.Minute)
	refreshDone := make(chan bool)

	if len(accessories) > 1 {
		bridge := hcAccessory.NewBridge(hcAccessory.Info{
			Name:             "Frigidaire HC",
			FirmwareRevision: version,
			Manufacturer:     "Nidito",
			Model:            "nidi-to/hc-frigidaire",
			ID:               1,
		})

		t, err := hc.NewIPTransport(config, bridge.Accessory, accessories...)

		if err != nil {
			log.Info.Panic(err)
		}

		hc.OnTermination(func() {
			<-t.Stop()
			refreshTicker.Stop()
			refreshDone <- true
		})

		t.Start()
	} else {
		t, err := hc.NewIPTransport(config, accessories[0])
		if err != nil {
			log.Info.Panic(err)
		}

		hc.OnTermination(func() {
			<-t.Stop()
			refreshTicker.Stop()
			refreshDone <- true
		})

		t.Start()
	}

	refresh := func() {
		log.Debug.Println("Refreshing telemetry")
		err := client.RefreshTelemetry()
		if err != nil {
			log.Info.Println("Failed to get telemetry")
			return
		}
	}

	go func(ticker *time.Ticker) {
		for {
			select {
			case <-refreshDone:
				return
			case <-ticker.C:
				refresh()
			}
		}
	}(refreshTicker)

}
