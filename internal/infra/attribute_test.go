package infra

import "testing"

func TestAttribute(t *testing.T) {
	tests := []struct {
		name   string
		log    string
		want   Reason
		wantOK bool
	}{
		{
			name:   "empty",
			log:    "",
			wantOK: false,
		},
		{
			name:   "agent channel closed",
			log:    "hudson.remoting.ChannelClosedException: Remote call on agent failed\nThe channel is closing down or has closed down",
			want:   AgentLost,
			wantOK: true,
		},
		{
			name:   "agent unexpected termination",
			log:    "java.io.IOException: Unexpected termination of the channel\n\tat hudson.remoting.Channel.call",
			want:   AgentLost,
			wantOK: true,
		},
		{
			name:   "agent went offline",
			log:    "ERROR: Agent went offline during the build",
			want:   AgentLost,
			wantOK: true,
		},
		{
			name:   "near miss application closed channel",
			log:    "java.nio.channels.ClosedChannelException: channel closed by the test client\nunexpected termination of the channel name parser",
			wantOK: false,
		},
		{
			name:   "oom killed",
			log:    "container status: OOMKilled",
			want:   OutOfMemory,
			wantOK: true,
		},
		{
			name:   "exit 137",
			log:    "script returned exit code 137",
			want:   OutOfMemory,
			wantOK: true,
		},
		{
			name:   "java oom",
			log:    "java.lang.OutOfMemoryError: Java heap space",
			want:   OutOfMemory,
			wantOK: true,
		},
		{
			name:   "near miss exit 1370",
			log:    "script returned exit code 1370\nnot an OOM event",
			wantOK: false,
		},
		{
			name:   "disk full",
			log:    "java.io.IOException: No space left on device",
			want:   DiskFull,
			wantOK: true,
		},
		{
			name:   "near miss disk wording",
			log:    "no space left in the device table",
			wantOK: false,
		},
		{
			name:   "docker daemon",
			log:    "Could not find a valid Docker environment: Cannot connect to the Docker daemon at unix:///var/run/docker.sock",
			want:   DockerUnavailable,
			wantOK: true,
		},
		{
			name:   "near miss docker registry",
			log:    "Cannot connect to the Docker registry\nConnected to the Docker daemon",
			wantOK: false,
		},
		{
			name: "dependency timeout",
			log: "" +
				"[ERROR] Failed to execute goal on project api: Could not resolve dependencies for project io.pipelineiq.lab:api:jar:0.1.0\n" +
				"Could not transfer artifact org.junit.jupiter:junit-jupiter:jar:6.1.1 from/to central (https://repo.maven.apache.org/maven2): Connect timed out",
			want:   DependencyFetch,
			wantOK: true,
		},
		{
			name:   "near miss missing artifact without timeout",
			log:    "[ERROR] Could not resolve dependencies for project io.pipelineiq.lab:api:jar:0.1.0: Could not find artifact io.pipelineiq.lab:missing:jar:9.9.9",
			wantOK: false,
		},
		{
			name:   "near miss test timeout without maven resolve",
			log:    "java.net.SocketTimeoutException: Read timed out\n\tat io.pipelineiq.lab.api.ApiTest.serves",
			wantOK: false,
		},
		{
			name:   "spot interruption is phase 2",
			log:    "Spot interruption notice: instance i-123 is being reclaimed",
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Attribute(tt.log)
			if ok != tt.wantOK || got != tt.want {
				t.Fatalf("Attribute() = (%q, %v), want (%q, %v)", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}
