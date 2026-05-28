package banner

import (
	"fmt"
)

// prints the version message
const version = "v0.0.1"

func PrintVersion() {
	fmt.Printf("Current urlflux version %s\n", version)
}

// Prints the Colorful banner
func PrintBanner() {
	banner := `
                __ ____ __            
  __  __ _____ / // __// /__  __ _  __
 / / / // ___// // /_ / // / / /| |/_/
/ /_/ // /   / // __// // /_/ /_>  <  
\__,_//_/   /_//_/  /_/ \__,_//_/|_|
`
	fmt.Printf("%s\n%45s\n\n", banner, "Current urlflux version "+version)
}
