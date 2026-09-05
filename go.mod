module koneko

go 1.27.1

require (
	github.com/Fiend3d/catatui v0.0.0
	github.com/alecthomas/chroma/v2 v2.15.0
	github.com/atotto/clipboard v0.1.4
	github.com/rivo/uniseg v0.4.7
	golang.org/x/sys v0.47.0
)

require (
	github.com/dlclark/regexp2 v1.11.4 // indirect
	golang.org/x/term v0.45.0 // indirect
)

// catatui is developed alongside koneko; point at the local checkout.
replace github.com/Fiend3d/catatui => ../catatui
