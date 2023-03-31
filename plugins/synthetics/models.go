package synthetics

type MonitorTag struct {
	Key    string   `json:"key"`
	Values []string `json:"values,omitempty"`
}

type MonitorEntity struct {
	GUID         string `json:"guid"`
	Name         string `json:"name"`
	MonitorType  string `json:"monitorType"`
	MonitoredURL string `json:"monitoredUrl"`
	GoldenTags   struct {
		Tags []MonitorTag `json:"tags"`
	} `json:"goldenTags"`
	Tags []MonitorTag `json:"tags"`
}

type MonitorStep struct {
	Ordinal int64    `json:"ordinal"`
	Type    string   `json:"type"`
	Values  []string `json:"values"`
}

type GetMonitorsResponse struct {
	Actor struct {
		EntitySearch struct {
			Results struct {
				Entities []MonitorEntity `json:"entities"`
			} `json:"results"`
		} `json:"entitySearch"`
	} `json:"actor"`
}

type GetStepsResponse struct {
	Actor struct {
		Account struct {
			Synthetics struct {
				Steps []MonitorStep `json:"steps"`
			} `json:"synthetics"`
		} `json:"account"`
	} `json:"actor"`
}

type GetScriptResponse struct {
	Actor struct {
		Account struct {
			Synthetics struct {
				Script struct {
					Text string `json:"text"`
				} `json:"script"`
			} `json:"synthetics"`
		} `json:"account"`
	} `json:"actor"`
}
