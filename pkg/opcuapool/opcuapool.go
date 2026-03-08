package opcuapool

import (
	"context"
	"fmt"
	"time"

	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/debug"
	"github.com/gopcua/opcua/monitor"
	"github.com/gopcua/opcua/ua"
)

const (
	// DefaultMaxDepthTree is the default maximum depth for the GetChildTree method to prevent infinite recursion
	DefaultMaxDepthTree = 20
	// DefaultStateChannelBufferSize is the default buffer size for the connection state channel
	DefaultStateChannelBufferSize = 10
)

type Options struct {
	mode          string
	policy        string
	certFile      string
	keyFile       string
	autoReconnect bool
	logOpcUa      bool
}

// NodeInfo represents information about an OPC UA node
type NodeInfo struct {
	NodeID      string // Node identifier
	NodeClass   string // Node class (Object, Variable, Method, etc.)
	BrowseName  string // Browse name
	DisplayName string // Display name
	Description string // Node description
	DataType    string // Data type name (e.g., "String", "Int32", "Double")
}

type OpcUaPool struct {
	endpoint            string
	ctx                 context.Context
	cancel              context.CancelFunc
	options             Options
	client              *opcua.Client
	dataChangeMessage   chan *monitor.DataChangeMessage
	nodeMonitor         *monitor.NodeMonitor
	checkEndpoint       bool
	stateChan           chan opcua.ConnState
	connectionState     bool
	endpoints           []*ua.EndpointDescription
	useOriginalEndpoint bool
	maxDepthTree        int
}

// New creates a new instance of OpcUaPool with the specified endpoint,
// data change message channel size, and endpoint checking option
func New(endpoint string, dataChangeMessageSize int, checkEndpoint bool) *OpcUaPool {
	pool := OpcUaPool{
		endpoint: endpoint,
		options: Options{
			mode:          "None",
			policy:        "None",
			certFile:      "",
			keyFile:       "",
			autoReconnect: false,
			logOpcUa:      false,
		},
		dataChangeMessage: make(chan *monitor.DataChangeMessage, dataChangeMessageSize),
		checkEndpoint:     checkEndpoint,
		stateChan:         make(chan opcua.ConnState, DefaultStateChannelBufferSize),
		maxDepthTree:      DefaultMaxDepthTree, // Default max depth for GetChildTree
	}

	return &pool
}

// WithSecMode sets the security mode for the OPC UA connection
func (pool *OpcUaPool) WithSecMode(mode string) *OpcUaPool {
	pool.options.mode = mode
	return pool
}

// WithOriginalEndpoint sets the flag to use the original endpoint URL instead of the one provided by the server's endpoint description
func (pool *OpcUaPool) WithOriginalEndpoint(useOriginal bool) *OpcUaPool {
	pool.useOriginalEndpoint = useOriginal
	return pool
}

// WithSecPolicy sets the security policy for the OPC UA connection
func (pool *OpcUaPool) WithSecPolicy(policy string) *OpcUaPool {
	pool.options.policy = policy
	return pool
}

// WithCerts sets the certificate and key file paths for secure OPC UA connections
func (pool *OpcUaPool) WithCerts(certFile string, keyFile string) *OpcUaPool {
	pool.options.certFile = certFile
	pool.options.keyFile = keyFile
	return pool
}

// WithAutoReconnect enables or disables automatic reconnection to the OPC UA server
func (pool *OpcUaPool) WithAutoReconnect(autoReconnect bool) *OpcUaPool {
	pool.options.autoReconnect = autoReconnect
	return pool
}

// WithConnectionState enables or disables tracking of the OPC UA connection state
func (pool *OpcUaPool) WithConnectionState(connectionState bool) *OpcUaPool {
	pool.connectionState = connectionState
	return pool
}

// WithLogOpcUa enables or disables OPC UA debug logging
func (pool *OpcUaPool) WithLogOpcUa(logOpcUa bool) *OpcUaPool {
	pool.options.logOpcUa = logOpcUa
	debug.Enable = logOpcUa
	return pool
}

// WithMaxDepthTree sets the maximum depth for the GetChildTree method to prevent infinite recursion
func (pool *OpcUaPool) WithMaxDepthTree(maxDepth int) *OpcUaPool {
	pool.maxDepthTree = maxDepth
	return pool
}

// GetEndpoints returns the available OPC UA endpoints
func (pool *OpcUaPool) GetEndpoints() ([]*ua.EndpointDescription, error) {
	if pool.endpoints != nil {
		return pool.endpoints, nil
	}
	endpoints, err := opcua.GetEndpoints(context.Background(), pool.endpoint)
	if err != nil {
		return nil, err
	}
	pool.endpoints = endpoints
	return endpoints, nil
}

// Done returns a channel that is closed when the OPC UA pool is closed,
// allowing callers to wait for shutdown
func (pool *OpcUaPool) Done() <-chan struct{} {
	return pool.ctx.Done()
}

// State returns the current OPC UA connection state
func (pool *OpcUaPool) State() opcua.ConnState {
	if pool.client == nil {
		return opcua.Closed
	}
	return pool.client.State()
}

// GetConnectionStateChannel returns the OPC UA connection state channel
func (pool *OpcUaPool) GetConnectionStateChannel() <-chan opcua.ConnState {
	return pool.stateChan
}

// Connect establishes a connection to the OPC UA server, optionally checking the available endpoints
// and security settings based on the configuration
func (pool *OpcUaPool) Connect() error {

	if pool.checkEndpoint {
		return pool.connectWithCheckEndpoint()
	}

	return pool.connectWithoutCheckEndpoint()
}

// connectWithoutCheckEndpoint establishes a connection to the OPC UA server
// without checking the available endpoints or security settings.
func (pool *OpcUaPool) connectWithoutCheckEndpoint() error {
	pool.ctx, pool.cancel = context.WithCancel(context.Background())
	var err error

	opts := []opcua.Option{
		opcua.SecurityMode(ua.MessageSecurityModeNone),
		opcua.AutoReconnect(pool.options.autoReconnect),
	}

	if pool.connectionState {
		opts = append(opts, opcua.StateChangedCh(pool.stateChan))
		// Start goroutine to drain state channel to prevent deadlock
		go func() {
			for {
				select {
				case <-pool.stateChan:
					// Drain the channel to prevent deadlock
				case <-pool.ctx.Done():
					return
				}
			}
		}()
	}

	pool.client, err = opcua.NewClient(pool.endpoint, opts...)

	if err != nil {
		return err
	}

	if err = pool.client.Connect(pool.ctx); err != nil {
		return err
	}
	return nil
}

// connectWithCheckEndpoint retrieves the available endpoints from the server,
// selects the appropriate one based on the configured security policy and mode,
// and establishes a connection using that endpoint
func (pool *OpcUaPool) connectWithCheckEndpoint() error {
	pool.ctx, pool.cancel = context.WithCancel(context.Background())
	endpoints, err := opcua.GetEndpoints(pool.ctx, pool.endpoint)
	if err != nil {
		return err
	}

	selectedEndpoint, err := opcua.SelectEndpoint(endpoints, pool.options.policy, ua.MessageSecurityModeFromString(pool.options.mode))
	pool.endpoints = endpoints

	if err != nil {
		return fmt.Errorf("no such endpoint: %s %v", pool.endpoint, err)
	}

	opts := []opcua.Option{
		opcua.SecurityPolicy(pool.options.policy),
		opcua.SecurityModeString(pool.options.mode),
		opcua.CertificateFile(pool.options.certFile),
		opcua.PrivateKeyFile(pool.options.keyFile),
		opcua.AuthAnonymous(),
		opcua.SecurityFromEndpoint(selectedEndpoint, ua.UserTokenTypeAnonymous),
		opcua.AutoReconnect(pool.options.autoReconnect),
	}

	if pool.connectionState {
		opts = append(opts, opcua.StateChangedCh(pool.stateChan))
		// Start goroutine to drain state channel to prevent deadlock
		go func() {
			for {
				select {
				case <-pool.stateChan:
					// Drain the channel to prevent deadlock
				case <-pool.ctx.Done():
					return
				}
			}
		}()
	}

	if pool.useOriginalEndpoint {
		pool.client, err = opcua.NewClient(pool.endpoint, opts...)
	} else {
		pool.client, err = opcua.NewClient(selectedEndpoint.EndpointURL, opts...)
	}

	if err != nil {
		return err
	}

	if err = pool.client.Connect(pool.ctx); err != nil {
		return err
	}
	return nil
}

// NewNodeMonitor initializes the NodeMonitor for the OPC UA client and sets the error handler
func (pool *OpcUaPool) NewNodeMonitor(errorHandler func(_ *opcua.Client, sub *monitor.Subscription, err error)) error {

	if pool.client == nil {
		return fmt.Errorf("client is not connected, call Connect() first")
	}

	var err error
	pool.nodeMonitor, err = monitor.NewNodeMonitor(pool.client)
	if err != nil {
		return err
	}

	pool.nodeMonitor.SetErrorHandler(errorHandler)

	return nil
}

// Close closes the OPC UA client connection and cancels the context
func (pool *OpcUaPool) Close() error {

	if pool.client == nil {
		return nil // Already closed or never connected
	}

	err := pool.client.Close(pool.ctx)
	if pool.cancel != nil {
		pool.cancel()
	}
	return err
}

// UnSubscribeAll unsubscribes from the given subscription
func (pool *OpcUaPool) UnSubscribeAll(subscription *monitor.Subscription) error {
	return subscription.Unsubscribe(pool.ctx)
}

// RemoveNodeFromSubscription removes the given node from the subscription
func (pool *OpcUaPool) RemoveNodeFromSubscription(subscription *monitor.Subscription, node string) error {
	return subscription.RemoveNodes(pool.ctx, node)
}

// Subscribe subscribes to the given nodes with the specified interval
func (pool *OpcUaPool) Subscribe(interval time.Duration, nodes []string) (*monitor.Subscription, chan *monitor.DataChangeMessage, error) {

	subscription, err := pool.nodeMonitor.ChanSubscribe(
		pool.ctx,
		&opcua.SubscriptionParameters{
			Interval: interval,
		},
		pool.dataChangeMessage,
		nodes...)

	if err != nil {
		return nil, nil, err
	}

	return subscription, pool.dataChangeMessage, nil
}

// WriteValue writes a value to the specified node ID
// The method automatically reads the node's DataType and converts the value to match,
// preventing type mismatch errors
func (pool *OpcUaPool) WriteValue(nodeID string, value any) error {
	if pool.client == nil {
		return fmt.Errorf("client is not connected, call Connect() first")
	}

	id, err := ua.ParseNodeID(nodeID)
	if err != nil {
		return fmt.Errorf("invalid node ID: %w", err)
	}

	// If value is already a Variant, use it directly
	var variant *ua.Variant
	switch v := value.(type) {
	case *ua.Variant:
		variant = v
	case ua.Variant:
		variant = &v
	default:
		// Strategy 1: Try to read DataType attribute
		node := pool.client.Node(id)
		attrs, err := node.Attributes(pool.ctx, ua.AttributeIDDataType)

		var dataTypeID *ua.NodeID
		if err == nil && len(attrs) > 0 && attrs[0].Status == ua.StatusOK {
			dataTypeID = attrs[0].Value.NodeID()
			if dataTypeID != nil {
				value = coerceValueToType(value, dataTypeID)
			}
		} else {
			// Strategy 2: If DataType read failed, try reading current value
			currentAttrs, err := node.Attributes(pool.ctx, ua.AttributeIDValue)
			if err == nil && len(currentAttrs) > 0 && currentAttrs[0].Status == ua.StatusOK {
				currentValue := currentAttrs[0].Value
				if currentValue != nil {
					// Try to match the type of current value
					value = matchValueType(value, currentValue)
				}
			}
		}

		// Create variant with explicit OPC UA type encoding
		if dataTypeID != nil && dataTypeID.Namespace() == 0 {
			variant = createVariantWithType(value, dataTypeID.IntID())
		} else {
			variant = ua.MustVariant(value)
		}
	}

	req := &ua.WriteRequest{
		NodesToWrite: []*ua.WriteValue{
			{
				NodeID:      id,
				AttributeID: ua.AttributeIDValue,
				Value: &ua.DataValue{
					EncodingMask: ua.DataValueValue, // Important: specify that we're only setting the value
					Value:        variant,
				},
			},
		},
	}

	resp, err := pool.client.Write(pool.ctx, req)
	if err != nil {
		return fmt.Errorf("write request failed: %w", err)
	}

	if resp == nil || len(resp.Results) == 0 {
		return fmt.Errorf("write request returned no results")
	}

	if resp.Results[0] != ua.StatusOK {
		return fmt.Errorf("write failed with status: %v", resp.Results[0])
	}

	return nil
}

// matchValueType tries to convert value to match the type of the current value
func matchValueType(newValue any, currentVariant *ua.Variant) any {
	if currentVariant == nil {
		return newValue
	}

	currentValue := currentVariant.Value()
	if currentValue == nil {
		return newValue
	}

	// Match the type based on current value's type
	switch currentValue.(type) {
	case float32:
		switch v := newValue.(type) {
		case float64:
			return float32(v)
		case int:
			return float32(v)
		case int32:
			return float32(v)
		case int64:
			return float32(v)
		}
	case float64:
		switch v := newValue.(type) {
		case float32:
			return float64(v)
		case int:
			return float64(v)
		case int32:
			return float64(v)
		case int64:
			return float64(v)
		}
	case int32:
		switch v := newValue.(type) {
		case float64:
			return int32(v)
		case float32:
			return int32(v)
		case int:
			return int32(v)
		case int64:
			return int32(v)
		}
	case uint32:
		switch v := newValue.(type) {
		case float64:
			return uint32(v)
		case float32:
			return uint32(v)
		case int:
			return uint32(v)
		case int64:
			return uint32(v)
		}
	case int16:
		switch v := newValue.(type) {
		case float64:
			return int16(v)
		case int:
			return int16(v)
		case int32:
			return int16(v)
		}
	case uint16:
		switch v := newValue.(type) {
		case float64:
			return uint16(v)
		case int:
			return uint16(v)
		case int32:
			return uint16(v)
		}
	case int64:
		switch v := newValue.(type) {
		case float64:
			return int64(v)
		case int:
			return int64(v)
		case int32:
			return int64(v)
		}
	case uint64:
		switch v := newValue.(type) {
		case float64:
			return uint64(v)
		case int:
			return uint64(v)
		case int32:
			return uint64(v)
		}
	}

	return newValue
}

// coerceValueToType converts a value to match the OPC UA DataType
func coerceValueToType(value any, dataTypeID *ua.NodeID) any {
	// Common OPC UA DataType node IDs (namespace 0)
	if dataTypeID.Namespace() != 0 {
		return value // Custom types, return as-is
	}

	// Handle numeric conversions
	switch v := value.(type) {
	case float64:
		switch dataTypeID.IntID() {
		case 10: // Float (Single precision)
			return float32(v)
		case 11: // Double
			return v
		case 6: // Int32
			return int32(v)
		case 7: // UInt32
			return uint32(v)
		case 4: // Int16
			return int16(v)
		case 5: // UInt16
			return uint16(v)
		case 2: // SByte
			return int8(v)
		case 3: // Byte
			return uint8(v)
		case 8: // Int64
			return int64(v)
		case 9: // UInt64
			return uint64(v)
		}
	case float32:
		switch dataTypeID.IntID() {
		case 11: // Double
			return float64(v)
		case 6: // Int32
			return int32(v)
		case 7: // UInt32
			return uint32(v)
		}
	case int:
		switch dataTypeID.IntID() {
		case 10: // Float
			return float32(v)
		case 11: // Double
			return float64(v)
		case 6: // Int32
			return int32(v)
		case 7: // UInt32
			return uint32(v)
		case 4: // Int16
			return int16(v)
		case 5: // UInt16
			return uint16(v)
		case 8: // Int64
			return int64(v)
		case 9: // UInt64
			return uint64(v)
		}
	case int32:
		switch dataTypeID.IntID() {
		case 10: // Float
			return float32(v)
		case 11: // Double
			return float64(v)
		case 7: // UInt32
			return uint32(v)
		}
	case int64:
		switch dataTypeID.IntID() {
		case 10: // Float
			return float32(v)
		case 11: // Double
			return float64(v)
		case 6: // Int32
			return int32(v)
		case 7: // UInt32
			return uint32(v)
		case 9: // UInt64
			return uint64(v)
		}
	}

	return value // Return original if no conversion needed
}

// GetChildNodes retrieves all child nodes for the given parent node ID
// nodeID: OPC UA node identifier string (e.g., "ns=2;i=1234")
// Returns: array of NodeInfo containing information about each child node
func (pool *OpcUaPool) GetChildNodes(nodeID string) ([]NodeInfo, error) {
	if pool.client == nil {
		return nil, fmt.Errorf("client is not connected, call Connect() first")
	}

	// Parse the node ID string
	id, err := ua.ParseNodeID(nodeID)
	if err != nil {
		return nil, fmt.Errorf("invalid node ID: %w", err)
	}

	node := pool.client.Node(id)

	// Get references using ReferencedNodes - more reliable than Browse
	// Use HierarchicalReferences (ID 33) to get all hierarchical children
	refs, err := node.ReferencedNodes(pool.ctx, 33, ua.BrowseDirectionForward, ua.NodeClassAll, true)
	if err != nil {
		return nil, fmt.Errorf("referenced nodes failed: %w", err)
	}

	if len(refs) == 0 {
		return []NodeInfo{}, nil
	}

	// Collect information for all child nodes
	nodeInfos := make([]NodeInfo, 0, len(refs))

	for _, refNode := range refs {
		// Read attributes for this node
		attrs, err := refNode.Attributes(pool.ctx,
			ua.AttributeIDNodeClass,
			ua.AttributeIDBrowseName,
			ua.AttributeIDDisplayName,
			ua.AttributeIDDescription,
			ua.AttributeIDDataType,
		)
		if err != nil {
			// Skip this node if we can't read attributes
			continue
		}

		if len(attrs) < 5 {
			// Skip if we don't have all attributes
			continue
		}

		nodeInfo := NodeInfo{
			NodeID: refNode.ID.String(),
		}

		// NodeClass
		if attrs[0].Status == ua.StatusOK {
			nodeInfo.NodeClass = ua.NodeClass(attrs[0].Value.Int()).String()
		}

		// BrowseName
		if attrs[1].Status == ua.StatusOK {
			nodeInfo.BrowseName = attrs[1].Value.String()
		}

		// DisplayName
		if attrs[2].Status == ua.StatusOK {
			if localizedText, ok := attrs[2].Value.Value().(ua.LocalizedText); ok {
				nodeInfo.DisplayName = localizedText.Text
			} else {
				nodeInfo.DisplayName = attrs[2].Value.String()
			}
		}

		// Description
		if attrs[3].Status == ua.StatusOK {
			if localizedText, ok := attrs[3].Value.Value().(ua.LocalizedText); ok {
				nodeInfo.Description = localizedText.Text
			}
		}

		// DataType - read BrowseName of the DataType node
		if attrs[4].Status == ua.StatusOK {
			dataTypeNodeID := attrs[4].Value.NodeID()
			if dataTypeNodeID != nil {
				dataTypeNode := pool.client.Node(dataTypeNodeID)
				dtAttrs, err := dataTypeNode.Attributes(pool.ctx, ua.AttributeIDBrowseName)
				if err == nil && len(dtAttrs) > 0 && dtAttrs[0].Status == ua.StatusOK {
					if qn, ok := dtAttrs[0].Value.Value().(*ua.QualifiedName); ok {
						nodeInfo.DataType = qn.Name
					}
				}
			}
		}

		nodeInfos = append(nodeInfos, nodeInfo)
	}

	return nodeInfos, nil
}

// GetChildTree retrieves all child nodes recursively for the given parent node ID
// nodeID: OPC UA node identifier string (e.g., "ns=2;i=1234")
// Returns: array of NodeInfo containing information about all child nodes at all levels
func (pool *OpcUaPool) GetChildTree(nodeID string) ([]NodeInfo, error) {
	return pool.GetChildTreeWithDepth(nodeID, pool.maxDepthTree)
}

// GetChildTreeWithDepth retrieves all child nodes recursively up to specified depth
func (pool *OpcUaPool) GetChildTreeWithDepth(nodeID string, maxDepth int) ([]NodeInfo, error) {
	if pool.client == nil {
		return nil, fmt.Errorf("client is not connected, call Connect() first")
	}

	var allNodes []NodeInfo
	visited := make(map[string]bool)

	// Use breadth-first search to avoid deep recursion
	type queueItem struct {
		nodeID string
		depth  int
	}

	queue := []queueItem{{nodeID: nodeID, depth: 0}}
	visited[nodeID] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		// Stop if we've reached max depth
		if current.depth >= maxDepth {
			continue
		}

		// Get children of current node
		children, err := pool.GetChildNodes(current.nodeID)
		if err != nil {
			// Skip this node if browse fails
			continue
		}

		// Add children to results
		allNodes = append(allNodes, children...)

		// Add unvisited children to queue
		for _, child := range children {
			if !visited[child.NodeID] {
				visited[child.NodeID] = true
				queue = append(queue, queueItem{
					nodeID: child.NodeID,
					depth:  current.depth + 1,
				})
			}
		}

		// Small delay to avoid overwhelming the server
		if len(queue) > 0 && current.depth > 0 && len(children) > 10 {
			time.Sleep(50 * time.Millisecond)
		}
	}

	return allNodes, nil
}

// createVariantWithType creates a Variant with explicit OPC UA type encoding
func createVariantWithType(value any, typeID uint32) *ua.Variant {
	// Use MustVariant which preserves the exact Go type
	// The key is that we've already converted the value to the correct Go type
	// (e.g., float32 instead of float64), so MustVariant will encode it correctly
	return ua.MustVariant(value)
}
