package blueprint

type Metric struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Delta string `json:"delta"`
}

type TimelineStep struct {
	Label  string `json:"label"`
	Status string `json:"status"`
	Date   string `json:"date"`
}

type WorkItem struct {
	ID       string         `json:"id"`
	Title    string         `json:"title"`
	Subtitle string         `json:"subtitle"`
	Status   string         `json:"status"`
	Due      string         `json:"due"`
	Amount   string         `json:"amount"`
	Progress int            `json:"progress"`
	Meta     []string       `json:"meta"`
	Timeline []TimelineStep `json:"timeline,omitempty"`
}

type Blueprint struct {
	ProductName     string     `json:"productName"`
	BackendStatus   string     `json:"backendStatus"`
	CustomerMetrics []Metric   `json:"customerMetrics"`
	AdminMetrics    []Metric   `json:"adminMetrics"`
	Orders          []WorkItem `json:"orders"`
	Shipping        []WorkItem `json:"shipping"`
	Calls           []WorkItem `json:"calls"`
}

func Current() Blueprint {
	return Blueprint{
		ProductName:   "ChakuChuri.pk",
		BackendStatus: "Platform API online",
		CustomerMetrics: []Metric{
			{Label: "Active orders", Value: "12", Delta: "3 ready this week"},
			{Label: "Pending quotes", Value: "7", Delta: "2 need approval"},
			{Label: "Payment due", Value: "Rs 184,000", Delta: "4 open balances"},
			{Label: "Shipments", Value: "28", Delta: "19 in transit"},
		},
		AdminMetrics: []Metric{
			{Label: "Online clients", Value: "118", Delta: "32 active now"},
			{Label: "Active calls", Value: "1", Delta: "Direct support"},
			{Label: "Quote pipeline", Value: "43", Delta: "11 waiting pricing"},
			{Label: "Delivery risk", Value: "6", Delta: "Needs action"},
		},
		Orders: []WorkItem{
			{
				ID:       "MFG-24091",
				Title:    "Damascus chef knife export batch",
				Subtitle: "ABC Export House",
				Status:   "Production",
				Due:      "Sep 15, 2026",
				Amount:   "Rs 420,000",
				Progress: 64,
				Meta:     []string{"D2 steel", "Full tang", "Leather sheath", "500 pcs"},
				Timeline: []TimelineStep{
					{Label: "Quote accepted", Status: "done", Date: "Aug 21"},
					{Label: "Deposit received", Status: "done", Date: "Aug 22"},
					{Label: "Production started", Status: "active", Date: "Today"},
					{Label: "Quality check", Status: "pending", Date: "Pending"},
				},
			},
			{
				ID:       "MFG-24092",
				Title:    "Outdoor axe sample set",
				Subtitle: "Northern Trading Co.",
				Status:   "Quoted",
				Due:      "Not confirmed",
				Amount:   "Rs 78,500",
				Progress: 18,
				Meta:     []string{"Sample order", "12 pcs", "Awaiting deposit"},
			},
		},
		Shipping: []WorkItem{
			{
				ID:       "SHIP-8102",
				Title:    "FedEx premium parcel",
				Subtitle: "Manufactured by ChakuChuri",
				Status:   "In transit",
				Due:      "2 days left",
				Amount:   "Rs 18,800",
				Progress: 72,
				Meta:     []string{"Zone 7", "4.8 kg", "Tracking added"},
			},
			{
				ID:       "SHIP-8103",
				Title:    "DHL outside-product shipment",
				Subtitle: "Customer-owned package",
				Status:   "Rate check",
				Due:      "Waiting admin confirmation",
				Amount:   "Estimate Rs 9,600",
				Progress: 24,
				Meta:     []string{"Not manufactured by us", "Express", "Zone pending"},
			},
		},
		Calls: []WorkItem{
			{
				ID:       "CALL-01",
				Title:    "Zain Exports",
				Subtitle: "Direct customer support call",
				Status:   "Completed",
				Due:      "Today",
				Amount:   "08:42",
				Progress: 100,
				Meta:     []string{"Incoming", "Answered", "Audio call"},
			},
			{
				ID:       "CALL-02",
				Title:    "Ali Traders",
				Subtitle: "Customer did not answer",
				Status:   "Missed",
				Due:      "Yesterday",
				Amount:   "No answer",
				Progress: 0,
				Meta:     []string{"Outgoing", "Call back available"},
			},
		},
	}
}
