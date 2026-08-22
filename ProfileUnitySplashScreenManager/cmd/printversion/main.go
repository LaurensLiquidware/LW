// Command printversion writes the application version to stdout, so build
// scripts can read the single source of truth rather than duplicating it.
package main

import (
	"fmt"

	"github.com/liquidware/profileunity-splashscreen-manager/internal/version"
)

func main() {
	fmt.Print(version.AppVersion)
}
