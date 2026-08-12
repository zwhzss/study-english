module study-english

go 1.26.4

require (
	github.com/wailsapp/wails/v2 v2.12.0
	study-english/server v0.0.0
)

replace study-english/server => ./server
