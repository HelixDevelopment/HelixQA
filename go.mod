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
	github.com/beevik/etree v1.5.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bits-and-blooms/bitset v1.24.4 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/crewjam/saml v0.5.1 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/go-ini/ini v1.67.0 // indirect
	github.com/go-jose/go-jose/v4 v4.1.4 // indirect
	github.com/jonboulle/clockwork v0.2.2 // indirect
	github.com/klauspost/compress v1.18.2 // indirect
	github.com/klauspost/cpuid/v2 v2.2.11 // indirect
	github.com/klauspost/crc32 v1.3.0 // indirect
	github.com/mattermost/xml-roundtrip-validator v0.1.0 // indirect
	github.com/minio/crc64nvme v1.1.1 // indirect
	github.com/minio/md5-simd v1.1.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/nats-io/nkeys v0.4.7 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/philhofer/fwd v1.2.0 // indirect
	github.com/pion/datachannel v1.6.0 // indirect
	github.com/pion/dtls/v3 v3.1.2 // indirect
	github.com/pion/ice/v4 v4.2.2 // indirect
	github.com/pion/interceptor v0.1.44 // indirect
	github.com/pion/logging v0.2.4 // indirect
	github.com/pion/mdns/v2 v2.1.0 // indirect
	github.com/pion/randutil v0.1.0 // indirect
	github.com/pion/rtcp v1.2.16 // indirect
	github.com/pion/rtp v1.10.1 // indirect
	github.com/pion/sctp v1.9.4 // indirect
	github.com/pion/sdp/v3 v3.0.18 // indirect
	github.com/pion/srtp/v3 v3.0.10 // indirect
	github.com/pion/stun/v3 v3.1.1 // indirect
	github.com/pion/transport/v4 v4.0.1 // indirect
	github.com/pion/turn/v4 v4.1.4 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/common v0.66.1 // indirect
	github.com/prometheus/procfs v0.16.1 // indirect
	github.com/rs/xid v1.6.0 // indirect
	github.com/russellhaering/goxmldsig v1.4.0 // indirect
	github.com/tinylib/msgp v1.6.1 // indirect
	github.com/wlynxg/anet v0.0.5 // indirect
	go.yaml.in/yaml/v2 v2.4.2 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.48.0 // indirect
	golang.org/x/net v0.50.0 // indirect
	golang.org/x/oauth2 v0.36.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	golang.org/x/time v0.10.0 // indirect
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

require (
	digital.vasic.llmsverifier v0.0.0
	github.com/coreos/go-oidc/v3 v3.18.0
	github.com/failsafe-go/failsafe-go v0.9.6
	github.com/godbus/dbus/v5 v5.2.2
	github.com/google/uuid v1.6.0
	github.com/gorilla/websocket v1.5.3
	github.com/minio/minio-go/v7 v7.0.100
	github.com/nats-io/nats.go v1.37.0
	github.com/otiai10/gosseract/v2 v2.4.1
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/philippgille/chromem-go v0.7.0
	github.com/pion/webrtc/v4 v4.2.11
	github.com/prometheus/client_golang v1.23.2
	github.com/prometheus/client_model v0.6.2
	gocv.io/x/gocv v0.43.0
)

replace digital.vasic.llmsverifier => ../LLMsVerifier
