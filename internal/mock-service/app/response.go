package app

type Response struct {
	Service     string            `json:"service"`
	Environment string            `json:"environment"`
	Timestamp   string            `json:"timestamp"`
	Path        string            `json:"path"`
	Method      string            `json:"method"`
	Headers     map[string]string `json:"headers"`
	Query       map[string]string `json:"query"`
	Host        string            `json:"host"`
}
