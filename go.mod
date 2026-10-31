module github.com/Vainillo1322/gdrive2proton

go 1.22.2

replace golang.org/x/sync => github.com/golang/sync v0.3.0

replace golang.org/x/crypto => github.com/golang/crypto v0.13.0

replace golang.org/x/exp => github.com/golang/exp v0.0.0-20230905200255-921286631fa9

replace golang.org/x/net => github.com/golang/net v0.15.0

replace golang.org/x/sys => github.com/golang/sys v0.12.0

replace golang.org/x/text => github.com/golang/text v0.13.0

replace golang.org/x/oauth2 => github.com/golang/oauth2 v0.21.0

replace golang.org/x/term => github.com/golang/term v0.12.0

replace golang.org/x/time => github.com/golang/time v0.3.0

require (
	github.com/henrybear327/Proton-API-Bridge v1.0.0
	github.com/henrybear327/go-proton-api v0.0.0-20230907193451-e563407504ce
	golang.org/x/term v0.12.0
)

require (
	github.com/ProtonMail/bcrypt v0.0.0-20211005172633-e235017c1baf // indirect
	github.com/ProtonMail/gluon v0.17.1-0.20230724134000-308be39be96e // indirect
	github.com/ProtonMail/go-crypto v0.0.0-20230828082145-3c4c8a2d2371 // indirect
	github.com/ProtonMail/go-mime v0.0.0-20230322103455-7d82a3887f2f // indirect
	github.com/ProtonMail/go-srp v0.0.7 // indirect
	github.com/ProtonMail/gopenpgp/v2 v2.7.3 // indirect
	github.com/PuerkitoBio/goquery v1.8.1 // indirect
	github.com/andybalholm/cascadia v1.3.2 // indirect
	github.com/bradenaw/juniper v0.13.1 // indirect
	github.com/cloudflare/circl v1.3.3 // indirect
	github.com/cronokirby/saferith v0.33.0 // indirect
	github.com/emersion/go-message v0.17.0 // indirect
	github.com/emersion/go-textwrapper v0.0.0-20200911093747-65d896831594 // indirect
	github.com/emersion/go-vcard v0.0.0-20230815062825-8fda7d206ec9 // indirect
	github.com/go-resty/resty/v2 v2.7.0 // indirect
	github.com/google/uuid v1.3.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/relvacode/iso8601 v1.3.0 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	golang.org/x/crypto v0.13.0 // indirect
	golang.org/x/exp v0.0.0-20230905200255-921286631fa9 // indirect
	golang.org/x/net v0.15.0 // indirect
	golang.org/x/sync v0.3.0 // indirect
	golang.org/x/sys v0.12.0 // indirect
	golang.org/x/text v0.13.0 // indirect
)

replace golang.org/x/arch => github.com/golang/arch v0.5.0

replace google.golang.org/protobuf => github.com/protocolbuffers/protobuf-go v1.31.0

replace gopkg.in/yaml.v3 => github.com/go-yaml/yaml/v3 v3.0.1

replace go.uber.org/goleak => github.com/uber-go/goleak v1.2.1

replace github.com/henrybear327/go-proton-api => ./third_party/go-proton-api
replace github.com/henrybear327/Proton-API-Bridge => ./third_party/Proton-API-Bridge
