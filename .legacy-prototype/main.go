package main

import (
	"context"
	"log"
	"os"

	"github.com/GhandyP/adk-go-enterprise-cookbook/internal/adkhelper"
)

func main() {
	if err := adkhelper.Run(
		context.Background(),
		os.Args[1:],
		"case_39_commercial_proposals_from_brief",
		"A text-only starting point for commercial proposals from a brief.",
		"Draft a commercial proposal from the brief and approved claims supplied by the user. Separate facts, assumptions, scope, and placeholders; do not invent prices or promises.",
	); err != nil {
		log.Fatal(err)
	}
}
