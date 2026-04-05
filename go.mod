module digital.vasic.helixqa

go 1.25.3

require (
	digital.vasic.challenges v0.0.0
	digital.vasic.containers v0.0.0-00010101000000-000000000000
	digital.vasic.docprocessor v0.0.0-00010101000000-000000000000
	digital.vasic.llmorchestrator v0.0.0-00010101000000-000000000000
	digital.vasic.visionengine v0.0.0-00010101000000-000000000000
	github.com/mattn/go-sqlite3 v1.14.37
	github.com/stretchr/testify v1.11.1
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_golang v1.23.2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.66.1 // indirect
	github.com/prometheus/procfs v0.16.1 // indirect
	go.yaml.in/yaml/v2 v2.4.2 // indirect
	golang.org/x/crypto v0.48.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
)

replace (
	digital.vasic.challenges => ../Challenges
	digital.vasic.containers => ../Containers
	digital.vasic.docprocessor => ../DocProcessor
	digital.vasic.llmorchestrator => ../LLMOrchestrator
	digital.vasic.llmprovider => ../LLMProvider
	digital.vasic.visionengine => ../VisionEngine
)
