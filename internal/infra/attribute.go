// Package infra decides whether a build failed because the machine did, not the test.
// It reads the log tail only. Spot events are Phase 2 and are not matched here.
package infra

import (
	"regexp"
	"strings"
)

// Reason is stored on build.infra_reason. The tokens are stable.
type Reason string

const (
	AgentLost         Reason = "AGENT_LOST"
	OutOfMemory       Reason = "OUT_OF_MEMORY"
	DiskFull          Reason = "DISK_FULL"
	DockerUnavailable Reason = "DOCKER_UNAVAILABLE"
	DependencyFetch   Reason = "DEPENDENCY_FETCH"
)

// exit137 matches "exit code 137" but not "exit code 1370".
var exit137 = regexp.MustCompile(`(?i)exit code 137\b`)

// Attribute returns the first matching rule. An empty tail is not an infra failure.
func Attribute(logTail string) (Reason, bool) {
	if logTail == "" {
		return "", false
	}
	switch {
	case agentLost(logTail):
		return AgentLost, true
	case outOfMemory(logTail):
		return OutOfMemory, true
	case strings.Contains(logTail, "No space left on device"):
		return DiskFull, true
	case dockerUnavailable(logTail):
		return DockerUnavailable, true
	case dependencyFetch(logTail):
		return DependencyFetch, true
	default:
		return "", false
	}
}

func agentLost(logTail string) bool {
	return strings.Contains(logTail, "hudson.remoting.ChannelClosedException") ||
		strings.Contains(logTail, "Unexpected termination of the channel") ||
		strings.Contains(logTail, "Agent went offline")
}

func outOfMemory(logTail string) bool {
	return strings.Contains(logTail, "OOMKilled") ||
		strings.Contains(logTail, "java.lang.OutOfMemoryError") ||
		exit137.MatchString(logTail)
}

func dockerUnavailable(logTail string) bool {
	return strings.Contains(logTail, "Cannot connect to the Docker daemon") ||
		strings.Contains(logTail, "Is the docker daemon running")
}

func dependencyFetch(logTail string) bool {
	resolution := strings.Contains(logTail, "Could not resolve dependencies") ||
		strings.Contains(logTail, "Could not transfer artifact")
	timeout := strings.Contains(logTail, "Connect timed out") ||
		strings.Contains(logTail, "Read timed out") ||
		strings.Contains(logTail, "Connection timed out") ||
		strings.Contains(logTail, "SocketTimeoutException")
	return resolution && timeout
}
