package otelinit

// Config allow you to set opentelemetry trace and metric,
// the variable of EnableMetric only used when Start()
type Config struct {
	AppName       string
	Host          string
	Port          int
	APIKey        string
	IsSecure      bool
	EnableMetrics bool
}
