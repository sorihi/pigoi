package templates

import "embed"

//go:embed all:go-api
var FS embed.FS

//go:embed all:go-api-feature
var FeatureFS embed.FS

//go:embed all:go-worker-feature
var WorkerFS embed.FS
