package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"opcua-simulator/pkg/opcuapool"
	"opcua-simulator/pkg/simulator"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// simulate runs the simulation loop, listening for simulated points and printing them until the context is canceled.
func simulate(ctx context.Context, opc *opcuapool.OpcUaPool, sim *simulator.Simulator) {
	for {
		select {
		case <-ctx.Done():
			log.Println("Simulation stopped")
			return
		case point := <-sim.Simulated():
			if err := opc.WriteValue(point.Key, point.Value); err != nil {
				log.Printf("Failed to write value for key '%s': %v", point.Key, err)
			}
		}
	}
}

func main() {
	endpoint := flag.String("endpoint", "", "host address of the OPC UA server")
	nodeID := flag.String("node", "i=84", "node id")
	certPath := flag.String("cert", "", "path to TLS certificate")
	keyPath := flag.String("key", "", "path to TLS private key")
	stepMs := flag.Int("step", 0, "simulation step in milliseconds")

	flag.Parse()

	if *endpoint == "" {
		fmt.Println("Error: Endpoint is required")
		flag.Usage()
		return
	}

	opc := opcuapool.New(*endpoint, 100, true)

	opc.WithSecPolicy("Basic256Sha256").
		WithSecMode("SignAndEncrypt").
		WithCerts(*certPath, *keyPath).
		WithAutoReconnect(true).
		WithLogOpcUa(false).
		WithOriginalEndpoint(true)

	if err := opc.Connect(); err != nil {
		log.Fatal(err)
	}

	defer opc.Close()

	log.Printf("Connected to OPC UA server %s\n", *endpoint)
	nodes, err := opc.GetChildTree(*nodeID)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Found %d nodes under '%s'\n", len(nodes), *nodeID)

	var wg sync.WaitGroup

	sim := simulator.NewSimulator(&wg)

	if *stepMs > 10 {
		sim.WithStepMs(time.Duration(*stepMs))
	}

	for _, node := range nodes {
		if node.DataType == "Float" {
			sim.AddPoint(
				node.NodeID,
				simulator.Config{
					Function: simulator.FunctionSine,
					Step:     0.01,
					Max:      50,
					Shift:    70,
					Time:     rand.Float64() * 2,
				},
			)
		} else if node.DataType == "String" {
			sim.AddPoint(
				node.NodeID,
				simulator.Config{
					Function: simulator.FunctionSelect,
					Values:   []any{"INNE", "UTE", "InvalidState"},
					Step:     0.1,
					Max:      100,
					Time:     rand.Float64(),
				},
			)
		} else {
			log.Printf("Unsupported data type '%s' for node '%s', skipping\n", node.DataType, node.NodeID)
		}
	}

	// Added TERMINAL signal handling to the context
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	wg.Go(func() {
		simulate(ctx, opc, sim)
	})

	sim.RunAsync(ctx)

	log.Printf("Simulation started with step %d. Press Ctrl+C to stop\n", sim.GetStepMs())
	wg.Wait()
}
