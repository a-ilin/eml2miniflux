module github.com/a-ilin/eml2miniflux

replace miniflux.app => ./sub/miniflux

replace github.com/sg3des/eml => github.com/a-ilin/go-eml v0.1.0

go 1.20

require (
	github.com/rylans/getlang v0.0.0-20201227074721-9e7f44ff8aa0
	github.com/sg3des/eml v0.1.0
	miniflux.app v0.0.0-20230417235842-d435e67a366b
)

require (
	github.com/PuerkitoBio/goquery v1.8.1 // indirect
	github.com/andybalholm/cascadia v1.3.2 // indirect
	github.com/lib/pq v1.10.9 // indirect
	github.com/paulrosania/go-charset v0.0.0-20190326053356-55c9d7a5834c // indirect
	github.com/stretchr/testify v1.8.2 // indirect
	github.com/yuin/goldmark v1.5.4 // indirect
	golang.org/x/crypto v0.8.0 // indirect
	golang.org/x/net v0.9.0 // indirect
	golang.org/x/text v0.9.0 // indirect
)
