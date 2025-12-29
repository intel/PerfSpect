module perfspect

go 1.25

replace (
	perfspect/internal/app => ./internal/app
	perfspect/internal/cpus => ./internal/cpus
	perfspect/internal/extract => ./internal/extract
	perfspect/internal/progress => ./internal/progress
	perfspect/internal/report => ./internal/report
	perfspect/internal/script => ./internal/script
	perfspect/internal/table => ./internal/table
	perfspect/internal/target => ./internal/target
	perfspect/internal/util => ./internal/util
	perfspect/internal/workflow => ./internal/workflow
)

require (
	github.com/casbin/govaluate v1.10.0
	github.com/deckarep/golang-set/v2 v2.8.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.23.2
	github.com/spf13/cobra v1.10.2
	github.com/spf13/pflag v1.0.10
	github.com/stretchr/testify v1.11.1
	github.com/xuri/excelize/v2 v2.10.0
	golang.org/x/term v0.38.0
	golang.org/x/text v0.32.0
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.67.4 // indirect
	github.com/prometheus/procfs v0.19.2 // indirect
	github.com/richardlehane/mscfb v1.0.5 // indirect
	github.com/richardlehane/msoleps v1.0.4 // indirect
	github.com/tiendc/go-deepcopy v1.7.2 // indirect
	github.com/xuri/efp v0.0.1 // indirect
	github.com/xuri/nfp v0.0.2-0.20250530014748-2ddeb826f9a9 // indirect
	go.yaml.in/yaml/v2 v2.4.3 // indirect
	golang.org/x/crypto v0.46.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
