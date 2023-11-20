package types

type EventIngest struct {
	EndTime        *int64            `json:"endTime,omitempty"`
	EntitySelector *string           `json:"entitySelector,omitempty"`
	EventType      string            `json:"eventType"`
	Properties     map[string]string `json:"properties"`
	StartTime      *int64            `json:"startTime"`
	Timeout        *int64            `json:"timeout,omitempty"`
	Title          string            `json:"title"`
}

type EventIngestResult struct {
	CorrelationId string `json:"correlationId"`
	Status        string `json:"status"`
}
type EventIngestResults struct {
	EventIngestResults []EventIngestResult `json:"eventIngestResults"`
	ReportCount        int                 `json:"reportCount"`
}

type EntitiesList struct {
	Entities    []Entity `json:"entities"`
	NextPageKey *string  `json:"nextPageKey"`
	PageSize    int      `json:"pageSize"`
	TotalCount  int      `json:"totalCount"`
}

type Entity struct {
	DisplayName string `json:"displayName"`
	EntityId    string `json:"entityId"`
	Type        string `json:"type"`
}
