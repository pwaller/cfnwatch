package main

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/awslabs/aws-sdk-go/aws"
	// "github.com/awslabs/aws-sdk-go/aws/awsutil"
	"github.com/awslabs/aws-sdk-go/service/cloudformation"
)

var svc = cloudformation.New(nil)

var mu sync.Mutex
var seenStacks = map[string]struct{}{}

func main() {
	args := os.Args[1:]
	if len(os.Args) < 1 {
		log.Fatal("usage: cfnwatch <stack name>")
	}
	stackName := args[0]

	t := time.Now().Add(-time.Hour / 2)
	ticker := time.NewTicker(550 * time.Millisecond)
	WatchStack(stackName, t, ticker)
}

func WatchStack(stackName string, start time.Time, ticker *time.Ticker) {
	if stackName == "" {
		return
	}

	mu.Lock()
	if _, seen := seenStacks[stackName]; seen {
		mu.Unlock()
		return
	}
	seenStacks[stackName] = struct{}{}
	mu.Unlock()

	fmt.Println("Watching stack:", stackName)

	t := start

	for {
		// Rate limit, each goroutine takes it in turn to do an API call.
		<-ticker.C

		// log.Println("TICK", stackName)

		resp, err := svc.DescribeStackEvents(&cloudformation.DescribeStackEventsInput{
			StackName: aws.String(stackName),
		})

		if awserr := aws.Error(err); awserr != nil {
			// A service error occurred.
			log.Println("Error:", awserr.Code, awserr.Message)
		} else if err != nil {
			// A non-service error occurred.
			panic(err)
		}

		for i := range resp.StackEvents {
			ev := resp.StackEvents[len(resp.StackEvents)-i-1]

			if ev.Timestamp.Before(t) || ev.Timestamp.Equal(t) {
				// Ignore events before last seen timestamp
				continue
			}

			t = *ev.Timestamp

			if *ev.ResourceType == "AWS::CloudFormation::Stack" {
				go WatchStack(*ev.PhysicalResourceID, t, ticker)
			}

			var s, r string
			s = *ev.ResourceStatus
			if ev.ResourceStatusReason != nil {
				r = *ev.ResourceStatusReason
			}
			ts := ev.Timestamp.Local().Format("15:04:05")

			if len(s) > 20 {
				s = s[:19] + "…"
			}

			fmt.Printf("%v %-25v %-20v %v\n", ts, *ev.LogicalResourceID, s, r)
		}
	}
}
