# Build the CLI with the same Go pin as go.mod, then copy it onto the inbound
# agent whose remoting version matches Jenkins 2.580.1.
FROM golang:1.27.1-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux \
    go build -trimpath -ldflags='-s -w' -o /out/pipelineiq ./cmd/pipelineiq

FROM maven:3.9.16-eclipse-temurin-21 AS maven

# Remoting 3386.v353e57a_1b_ea_0 is the version in the Jenkins 2.580.1 pom.
# -3 is the newest image revision of that remoting. Tag confirmed on Docker Hub.
FROM jenkins/inbound-agent:3386.v353e57a_1b_ea_0-3-jdk21

USER root
RUN apt-get update \
 && apt-get install -y --no-install-recommends curl \
 && rm -rf /var/lib/apt/lists/*
COPY --from=maven /usr/share/maven /usr/share/maven
COPY --from=build /usr/local/go /usr/local/go
COPY --from=build /out/pipelineiq /usr/local/bin/pipelineiq
COPY jenkins/agent/entrypoint.sh /usr/local/bin/pipelineiq-agent
RUN chmod 755 /usr/local/bin/pipelineiq /usr/local/bin/pipelineiq-agent \
 && ln -sf /usr/share/maven/bin/mvn /usr/local/bin/mvn
ENV MAVEN_HOME=/usr/share/maven
ENV PATH="/usr/local/go/bin:/usr/share/maven/bin:${PATH}"
USER jenkins

ENTRYPOINT ["/usr/local/bin/pipelineiq-agent"]
