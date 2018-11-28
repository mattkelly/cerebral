package main

import (
	"flag"
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/containership/cerebral/cmd/autoscalingengine/app"

	// Register the autoscaling engine that needs to be initialized
	_ "github.com/containership/cerebral/pkg/autoscalingengine/containership"
)

func main() {
	// We don't have any of our own flags to parse, but k8s packages want to
	// use glog and we have to pass flags to that to configure it to behave
	// in a sane way.
	flag.Parse()

	if err := app.Run(wait.NeverStop); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
