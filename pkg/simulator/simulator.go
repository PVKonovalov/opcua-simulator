package simulator

import (
	"context"
	"math"
	"math/rand"
	"sync"
	"time"
)

type Function int

const (
	FunctionRandom Function = iota
	FunctionSine
	FunctionSquare
	FunctionTriangle
	FunctionConst
	FunctionSelect
)

const DefaultStep time.Duration = 1000

type Config struct {
	Function Function // The Function to generate the value
	Time     float64  // The Time at which the value is generated
	Step     float64  // The time Step for the simulation
	Max      float64  // The maximum value of the signal
	Shift    float64
	Values   []any // The Values to select from for the select Function
}

type Point struct {
	config Config
	Value  any
}

type SimulatedPoint struct {
	Key   string
	Value any
}

type Simulator struct {
	points   map[string]*Point // The points to simulate
	output   chan SimulatedPoint
	wgGlobal *sync.WaitGroup // Wait group for goroutines
	stepMs   time.Duration
}

func NewSimulator(wg *sync.WaitGroup) *Simulator {
	return &Simulator{
		points:   make(map[string]*Point),
		output:   make(chan SimulatedPoint, 100), // Buffered channel to hold simulated points
		wgGlobal: wg,
		stepMs:   DefaultStep,
	}
}

// WithStepMs sets the step duration for the simulator
func (s *Simulator) WithStepMs(stepMs time.Duration) *Simulator {
	s.stepMs = stepMs
	return s
}

// GetStepMs returns the current step duration of the simulator
func (s *Simulator) GetStepMs() time.Duration {
	return s.stepMs
}

// Simulated returns a channel that can be used to receive simulated points
func (s *Simulator) Simulated() <-chan SimulatedPoint {
	return s.output
}

// AddPoint adds a new point to the simulator with the given key and configuration
func (s *Simulator) AddPoint(key string, point Config) {
	s.points[key] = &Point{config: point}
}

// step simulates the next Value for each point based on its configuration
func (p *Point) step() *Point {
	switch p.config.Function {
	case FunctionRandom:
		// Generate a random value between 0 and max
		p.Value = float64(rand.Intn(int(p.config.Max)))
	case FunctionSine:
		// Generate a sine wave value based on time and step
		p.Value = p.config.Shift + p.config.Max*math.Sin(p.config.Time)
		p.config.Time += p.config.Step
	case FunctionSquare:
		// Generate a square wave value based on time and step
		if int(p.config.Time/p.config.Step)%2 == 0 {
			p.Value = p.config.Max
		} else {
			p.Value = 0.0
		}
		p.config.Time += p.config.Step
	case FunctionTriangle:
		// Generate a triangle wave value based on time and step
		t := math.Mod(p.config.Time, 2*p.config.Step)
		if t < p.config.Step {
			p.Value = (p.config.Max / p.config.Step) * t
		} else {
			p.Value = (p.config.Max / p.config.Step) * (2*p.config.Step - t)
		}
		p.config.Time += p.config.Step
	case FunctionConst:
		// Return a constant value (the first value in the values slice)
		if len(p.config.Values) > 0 {
			p.Value = p.config.Values[0]
		} else {
			p.Value = 0.0
		}
	case FunctionSelect:
		// Select a value from the values slice based on time and step
		if len(p.config.Values) > 0 {
			index := int(p.config.Time) % len(p.config.Values)
			p.Value = p.config.Values[index]
			p.config.Time += p.config.Step
		}
	default:
		p.Value = 0.0
	}
	return p
}

// step simulates the next value for each point in the simulator
func (s *Simulator) step() {
	for key, point := range s.points {
		s.points[key] = point.step()
	}
}

// Simulate runs the simulation for a specified number of steps and sends the simulated points to the output channel
func (s *Simulator) Simulate(ctx context.Context) {
	ticker := time.NewTicker(s.stepMs * time.Millisecond) // Adjust the tick duration as needed
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for key, point := range s.points {
				s.points[key] = point.step()
				s.output <- SimulatedPoint{
					Key:   key,
					Value: point.Value,
				}
			}
		}
	}
}

// RunAsync starts the simulation in a separate goroutine and returns a channel to receive the simulated points
func (s *Simulator) RunAsync(ctx context.Context) <-chan SimulatedPoint {
	s.wgGlobal.Go(func() {
		s.Simulate(ctx)
	})
	return s.Simulated()
}
