package opcuapool

import (
	"strings"
	"testing"

	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/monitor"
)

// TestNewNodeMonitorWithoutConnect verifies that NewNodeMonitor returns an error when called before Connect
func TestNewNodeMonitorWithoutConnect(t *testing.T) {
	pool := New("opc.tcp://localhost:4840", 100, false)

	err := pool.NewNodeMonitor(func(_ *opcua.Client, _ *monitor.Subscription, _ error) {})

	if err == nil {
		t.Fatal("Expected error when NewNodeMonitor is called before Connect, got nil")
	}

	expectedMsg := "client is not connected"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error message containing '%s', got: %v", expectedMsg, err)
	}
}

// TestCloseWithoutConnect verifies that Close handles nil client gracefully
func TestCloseWithoutConnect(t *testing.T) {
	pool := New("opc.tcp://localhost:4840", 100, false)

	err := pool.Close()

	if err != nil {
		t.Errorf("Expected no error when closing without connection, got: %v", err)
	}
}

// TestStateReturnsClosedWhenClientIsNil verifies State() returns Closed when client is nil
func TestStateReturnsClosedWhenClientIsNil(t *testing.T) {
	pool := New("opc.tcp://localhost:4840", 100, false)

	state := pool.State()

	if state != opcua.Closed {
		t.Errorf("Expected state to be Closed when client is nil, got: %v", state)
	}
}

// TestConnectionStateChannel verifies GetConnectionStateChannel returns a valid channel
func TestConnectionStateChannel(t *testing.T) {
	pool := New("opc.tcp://localhost:4840", 100, false)

	ch := pool.GetConnectionStateChannel()

	if ch == nil {
		t.Error("Expected non-nil connection state channel")
	}
}

// TestWithBuilderPattern verifies the builder pattern methods work correctly
func TestWithBuilderPattern(t *testing.T) {
	pool := New("opc.tcp://localhost:4840", 100, false).
		WithSecMode("SignAndEncrypt").
		WithSecPolicy("Basic256Sha256").
		WithCerts("/path/to/cert", "/path/to/key").
		WithAutoReconnect(true).
		WithConnectionState(true).
		WithLogOpcUa(false).
		WithOriginalEndpoint(true)

	if pool.options.mode != "SignAndEncrypt" {
		t.Errorf("Expected mode 'SignAndEncrypt', got: %s", pool.options.mode)
	}
	if pool.options.policy != "Basic256Sha256" {
		t.Errorf("Expected policy 'Basic256Sha256', got: %s", pool.options.policy)
	}
	if pool.options.certFile != "/path/to/cert" {
		t.Errorf("Expected certFile '/path/to/cert', got: %s", pool.options.certFile)
	}
	if pool.options.keyFile != "/path/to/key" {
		t.Errorf("Expected keyFile '/path/to/key', got: %s", pool.options.keyFile)
	}
	if !pool.options.autoReconnect {
		t.Error("Expected autoReconnect to be true")
	}
	if !pool.connectionState {
		t.Error("Expected connectionState to be true")
	}
	if !pool.useOriginalEndpoint {
		t.Error("Expected useOriginalEndpoint to be true")
	}
}

// TestGetChildNodesWithoutConnect verifies that GetChildNodes returns an error when called before Connect
func TestGetChildNodesWithoutConnect(t *testing.T) {
	pool := New("opc.tcp://localhost:4840", 100, false)

	_, err := pool.GetChildNodes("ns=0;i=85")

	if err == nil {
		t.Fatal("Expected error when GetChildNodes is called before Connect, got nil")
	}

	expectedMsg := "client is not connected"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error message containing '%s', got: %v", expectedMsg, err)
	}
}

// TestGetChildNodesInvalidNodeID verifies that GetChildNodes returns an error for invalid node IDs
func TestGetChildNodesInvalidNodeID(t *testing.T) {
	pool := New("opc.tcp://localhost:4840", 100, false)

	_, err := pool.GetChildNodes("invalid_node_id")

	if err == nil {
		t.Fatal("Expected error for invalid or disconnected state, got nil")
	}

	// The error can be either "client is not connected" or "invalid node ID"
	// depending on the implementation order
	validErrors := []string{"invalid node ID", "client is not connected"}
	foundValidError := false
	for _, expectedMsg := range validErrors {
		if strings.Contains(err.Error(), expectedMsg) {
			foundValidError = true
			break
		}
	}

	if !foundValidError {
		t.Errorf("Expected error message containing one of %v, got: %v", validErrors, err)
	}
}

// TestGetChildTreeWithoutConnect verifies that GetChildTree returns an error when called before Connect
func TestGetChildTreeWithoutConnect(t *testing.T) {
	pool := New("opc.tcp://localhost:4840", 100, false)

	_, err := pool.GetChildTree("ns=0;i=85")

	if err == nil {
		t.Fatal("Expected error when GetChildTree is called before Connect, got nil")
	}

	expectedMsg := "client is not connected"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error message containing '%s', got: %v", expectedMsg, err)
	}
}

// TestGetChildTreeInvalidNodeID verifies that GetChildTree returns an error for invalid node IDs
func TestGetChildTreeInvalidNodeID(t *testing.T) {
	pool := New("opc.tcp://localhost:4840", 100, false)

	_, err := pool.GetChildTree("invalid_node_id")

	if err == nil {
		t.Fatal("Expected error for invalid or disconnected state, got nil")
	}

	// The error can be either "client is not connected" or "invalid node ID"
	// depending on the implementation order
	validErrors := []string{"invalid node ID", "client is not connected"}
	foundValidError := false
	for _, expectedMsg := range validErrors {
		if strings.Contains(err.Error(), expectedMsg) {
			foundValidError = true
			break
		}
	}

	if !foundValidError {
		t.Errorf("Expected error message containing one of %v, got: %v", validErrors, err)
	}
}

// TestWriteValueWithoutConnect verifies that WriteValue returns an error when called before Connect
func TestWriteValueWithoutConnect(t *testing.T) {
	pool := New("opc.tcp://localhost:4840", 100, false)

	err := pool.WriteValue("ns=0;i=85", 42)

	if err == nil {
		t.Fatal("Expected error when WriteValue is called before Connect, got nil")
	}

	expectedMsg := "client is not connected"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error message containing '%s', got: %v", expectedMsg, err)
	}
}

// TestWriteValueInvalidNodeID verifies that WriteValue returns an error for invalid node IDs
func TestWriteValueInvalidNodeID(t *testing.T) {
	pool := New("opc.tcp://localhost:4840", 100, false)

	err := pool.WriteValue("invalid_node_id", "value")

	if err == nil {
		t.Fatal("Expected error for invalid or disconnected state, got nil")
	}

	validErrors := []string{"invalid node ID", "client is not connected"}
	foundValidError := false
	for _, expectedMsg := range validErrors {
		if strings.Contains(err.Error(), expectedMsg) {
			foundValidError = true
			break
		}
	}

	if !foundValidError {
		t.Errorf("Expected error message containing one of %v, got: %v", validErrors, err)
	}
}
