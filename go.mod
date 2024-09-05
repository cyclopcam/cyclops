module github.com/cyclopcam/cyclops

go 1.22.0

//replace github.com/go-chi/httprate => ../httprate

//replace github.com/bluenviron/gortsplib/v4 => ../gortsplib

//replace github.com/bmharper/ringbuffer => ../ringbuffer

require (
	cloud.google.com/go/logging v1.11.0
	cloud.google.com/go/storage v1.43.0
	github.com/BurntSushi/migration v0.0.0-20140125045755-c45b897f1335
	github.com/akamensky/argparse v1.4.0
	github.com/asticode/go-astits v1.13.0
	github.com/bluenviron/gortsplib/v4 v4.10.3
	github.com/bluenviron/mediacommon v1.12.2
	github.com/bmharper/cimg/v2 v2.0.8
	github.com/bmharper/flatbush-go v1.1.1
	github.com/bmharper/ringbuffer v1.1.2
	github.com/bmharper/tiledinference v1.0.3
	github.com/chewxy/math32 v1.11.0
	github.com/coreos/go-systemd v0.0.0-20191104093116-d3cd4ed1dbcf
	github.com/go-chi/httprate v0.14.0
	github.com/gorilla/websocket v1.5.3
	github.com/julienschmidt/httprouter v1.3.0
	github.com/lib/pq v1.10.9
	github.com/mattn/go-sqlite3 v1.14.22
	github.com/pion/rtp v1.8.9
	github.com/stretchr/testify v1.9.0
	golang.org/x/crypto v0.26.0
	golang.zx2c4.com/wireguard/wgctrl v0.0.0-20230429144221-925a1e7659e6
	gorm.io/driver/postgres v1.5.9
	gorm.io/driver/sqlite v1.5.6
	gorm.io/gorm v1.25.11
)

require (
	cloud.google.com/go v0.115.1 // indirect
	cloud.google.com/go/auth v0.9.1 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.4 // indirect
	cloud.google.com/go/compute/metadata v0.5.0 // indirect
	cloud.google.com/go/iam v1.2.0 // indirect
	cloud.google.com/go/longrunning v0.6.0 // indirect
	github.com/asticode/go-astikit v0.43.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cyclopcam/logs v0.0.0-20240905191637-c7ae6d2e0e38 // indirect
	github.com/cyclopcam/safewg v0.0.0-20240905192045-200ca27410c9 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dsoprea/go-exif/v3 v3.0.1 // indirect
	github.com/dsoprea/go-iptc v0.0.0-20200610044640-bc9ca208b413 // indirect
	github.com/dsoprea/go-jpeg-image-structure/v2 v2.0.0-20221012074422-4f3f7e934102 // indirect
	github.com/dsoprea/go-logging v0.0.0-20200710184922-b02d349568dd // indirect
	github.com/dsoprea/go-photoshop-info-format v0.0.0-20200610045659-121dd752914d // indirect
	github.com/dsoprea/go-utility/v2 v2.0.0-20221003172846-a3e1774ef349 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-errors/errors v1.5.1 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-xmlfmt/xmlfmt v1.1.2 // indirect
	github.com/golang/geo v0.0.0-20230421003525-6adc56603217 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/s2a-go v0.1.8 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.2 // indirect
	github.com/googleapis/gax-go/v2 v2.13.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.5.5 // indirect
	github.com/jackc/puddle/v2 v2.2.1 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/josharian/native v1.1.0 // indirect
	github.com/kr/text v0.1.0 // indirect
	github.com/mdlayher/genetlink v1.3.2 // indirect
	github.com/mdlayher/netlink v1.7.2 // indirect
	github.com/mdlayher/socket v0.5.1 // indirect
	github.com/pion/randutil v0.1.0 // indirect
	github.com/pion/rtcp v1.2.14 // indirect
	github.com/pion/sdp/v3 v3.0.9 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rogpeppe/go-internal v1.12.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.52.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.53.0 // indirect
	go.opentelemetry.io/otel v1.28.0 // indirect
	go.opentelemetry.io/otel/metric v1.28.0 // indirect
	go.opentelemetry.io/otel/trace v1.28.0 // indirect
	golang.org/x/image v0.19.0 // indirect
	golang.org/x/net v0.28.0 // indirect
	golang.org/x/oauth2 v0.22.0 // indirect
	golang.org/x/sync v0.8.0 // indirect
	golang.org/x/sys v0.24.0 // indirect
	golang.org/x/text v0.17.0 // indirect
	golang.org/x/time v0.6.0 // indirect
	golang.zx2c4.com/wireguard v0.0.0-20231211153847-12269c276173 // indirect
	google.golang.org/api v0.194.0 // indirect
	google.golang.org/genproto v0.0.0-20240823204242-4ba0660f739c // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240823204242-4ba0660f739c // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240823204242-4ba0660f739c // indirect
	google.golang.org/grpc v1.65.0 // indirect
	google.golang.org/protobuf v1.34.2 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
