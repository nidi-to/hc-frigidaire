package main

import "math"

func fahrenheitToCelcius(fahrenheit int) float64 {
	// 10 is intentional, since the cool connect api returns centi-fahrenheit
	return math.Round((float64(fahrenheit/10) - 32.0) * 5.0 / 9.0)
}
