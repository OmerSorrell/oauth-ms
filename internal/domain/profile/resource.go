package profile

// Resource is the normalized provider-resource unit.
//
// For GitHub this is a repository (Metric=stars, MetricLabel="stars"). A future Slack
// provider could emit channels (Metric=members, MetricLabel="members"). The output
// shape stays stable for clients regardless of provider.
type Resource struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Metric      int64  `json:"metric"`
	MetricLabel string `json:"metric_label"`
}
