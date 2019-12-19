module github.com/Omnition/omnition-opentelemetry-collector

go 1.12

require (
	contrib.go.opencensus.io/exporter/ocagent v0.6.0
	contrib.go.opencensus.io/resource v0.1.2
	github.com/StackExchange/wmi v0.0.0-20190523213315-cbe66965904d // indirect
	github.com/census-instrumentation/opencensus-proto v0.2.2
	github.com/client9/misspell v0.3.4
	github.com/dropbox/godropbox v0.0.0-20190501155911-5749d3b71cbe // indirect
	github.com/facebookgo/stack v0.0.0-20160209184415-751773369052 // indirect
	github.com/facebookgo/stackerr v0.0.0-20150612192056-c2fcf88613f4 // indirect
	github.com/go-ole/go-ole v1.2.4 // indirect
	github.com/gogo/protobuf v1.3.1
	github.com/golang/protobuf v1.3.2
	github.com/google/addlicense v0.0.0-20190510175307-22550fa7c1b0
	github.com/google/uuid v1.1.1
	github.com/grpc-ecosystem/grpc-gateway v1.11.1
	github.com/jaegertracing/jaeger v1.15.1
	github.com/jstemmer/go-junit-report v0.9.1
	github.com/juju/errors v0.0.0-20190930114154-d42613fe1ab9 // indirect
	github.com/juju/loggo v0.0.0-20190526231331-6e530bcce5d8 // indirect
	github.com/juju/testing v0.0.0-20191001232224-ce9dec17d28b // indirect
	github.com/omnition/gogoproto-rewriter v0.0.0-20190723134119-239e2d24817f
	github.com/omnition/opencensus-go-exporter-kinesis v0.3.3-0.20190919185502-7031b700cdfe
	github.com/open-telemetry/opentelemetry-collector v0.2.1-0.20191209163440-5d463fe48816
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kinesisexporter v0.0.0-20191216151958-b96fcb08e351
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter v0.0.0-20191216151958-b96fcb08e351
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter v0.0.0-20191216151958-b96fcb08e351
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sapmreceiver v0.0.0-20191216151958-b96fcb08e351
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver v0.0.0-20191218190817-7efaeccd8680
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinscribereceiver v0.0.0-20191216151958-b96fcb08e351
	github.com/rs/cors v1.6.0
	github.com/shirou/gopsutil v2.19.9+incompatible
	github.com/signalfx/golib v2.5.1+incompatible
	github.com/smartystreets/goconvey v1.6.4 // indirect
	github.com/soheilhy/cmux v0.1.4
	github.com/stretchr/testify v1.4.0
	go.opencensus.io v0.22.2
	go.uber.org/zap v1.13.0
	golang.org/x/lint v0.0.0-20191125180803-fdd1cda4f05f
	golang.org/x/net v0.0.0-20191206103017-1ddd1de85cb0
	golang.org/x/tools v0.0.0-20191205225056-3393d29bb9fe
	google.golang.org/genproto v0.0.0-20191205163323-51378566eb59
	google.golang.org/grpc v1.25.1
	gopkg.in/mgo.v2 v2.0.0-20190816093944-a6b53ec6cb22 // indirect
	honnef.co/go/tools v0.0.1-2019.2.3
	k8s.io/api v0.0.0-20190813020757-36bff7324fb7
	k8s.io/apimachinery v0.0.0-20190809020650-423f5d784010
	k8s.io/client-go v12.0.0+incompatible
)

replace contrib.go.opencensus.io/exporter/ocagent => github.com/omnition/opencensus-go-exporter-ocagent v0.6.0-omnition

replace github.com/census-instrumentation/opencensus-proto => github.com/omnition/opencensus-proto v0.2.1-gogo-unary

replace git.apache.org/thrift.git => github.com/apache/thrift v0.12.0
