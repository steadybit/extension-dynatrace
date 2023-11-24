package types

type CreateMaintenanceWindowRequest struct {
	SchemaId string            `json:"schemaId"`
	Scope    string            `json:"scope"`
	Value    MaintenanceWindow `json:"value"`
}

type CreateMaintenanceWindowResponse struct {
	Code     int    `json:"code"`
	ObjectId string `json:"objectId"`
}

type MaintenanceWindow struct {
	Enabled           bool                               `json:"enabled"`
	GeneralProperties MaintenanceWindowGeneralProperties `json:"generalProperties"`
	Schedule          MaintenanceWindowSchedule          `json:"schedule"`
}

type MaintenanceWindowGeneralProperties struct {
	Name                             string `json:"name"`
	Description                      string `json:"description"`
	MaintenanceType                  string `json:"maintenanceType"`
	Suppression                      string `json:"suppression"`
	DisableSyntheticMonitorExecution bool   `json:"disableSyntheticMonitorExecution"`
}

type MaintenanceWindowSchedule struct {
	ScheduleType   string                                  `json:"scheduleType"`
	OnceRecurrence MaintenanceWindowScheduleOnceRecurrence `json:"onceRecurrence"`
}
type MaintenanceWindowScheduleOnceRecurrence struct {
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	TimeZone  string `json:"timeZone"`
}

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
