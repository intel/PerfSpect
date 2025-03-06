module perfspect

go 1.23.0

replace (
	perfspect/internal/common => ./internal/common
	perfspect/internal/cpudb => ./internal/cpudb
	perfspect/internal/progress => ./internal/progress
	perfspect/internal/report => ./internal/report
	perfspect/internal/script => ./internal/script
	perfspect/internal/target => ./internal/target
	perfspect/internal/util => ./internal/util
)

require (
	github.com/Knetic/govaluate v3.0.0+incompatible
	github.com/deckarep/golang-set/v2 v2.7.0
	github.com/spf13/cobra v1.9.1
	github.com/spf13/pflag v1.0.6
	github.com/xuri/excelize/v2 v2.9.0
	golang.org/x/exp v0.0.0-20241217172543-b2144cdd0a67
	golang.org/x/term v0.30.0
	golang.org/x/text v0.22.0
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/richardlehane/mscfb v1.0.4 // indirect
	github.com/richardlehane/msoleps v1.0.4 // indirect
	github.com/xuri/efp v0.0.0-20241211021726-c4e992084aa6 // indirect
	github.com/xuri/nfp v0.0.0-20240318013403-ab9948c2c4a7 // indirect
	golang.org/x/crypto v0.31.0 // indirect
	golang.org/x/net v0.33.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
)
