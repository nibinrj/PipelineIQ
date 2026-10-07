#!/usr/bin/env bash
# Brings the compose stack up through tasks.ps1, runs one lab build on the
# agent, and writes the API response and docker stats for the prompt's done check.
set -euo pipefail

root=$(cd "$(dirname "$0")/../.." && pwd)
cd "$root"

write_lab() {
  mkdir -p ci-lab/core/src/main/java/io/pipelineiq/lab/core
  mkdir -p ci-lab/core/src/test/java/io/pipelineiq/lab/core
  mkdir -p ci-lab/api/src/main/java/io/pipelineiq/lab/api
  mkdir -p ci-lab/api/src/test/java/io/pipelineiq/lab/api
  mkdir -p ci-lab/flaky-lab/src/test/java/io/pipelineiq/lab/flaky
  cat > ci-lab/pom.xml << 'EOF'
<project xmlns="http://maven.apache.org/POM/4.0.0">
  <modelVersion>4.0.0</modelVersion>
  <groupId>io.pipelineiq.lab</groupId>
  <artifactId>pipelineiq-lab</artifactId>
  <version>0.1.0</version>
  <packaging>pom</packaging>
  <modules>
    <module>core</module>
    <module>api</module>
    <module>flaky-lab</module>
  </modules>
  <properties>
    <maven.compiler.release>21</maven.compiler.release>
    <project.build.sourceEncoding>UTF-8</project.build.sourceEncoding>
    <junit.version>6.1.1</junit.version>
  </properties>
  <dependencyManagement>
    <dependencies>
      <dependency>
        <groupId>org.junit.jupiter</groupId>
        <artifactId>junit-jupiter</artifactId>
        <version>${junit.version}</version>
      </dependency>
    </dependencies>
  </dependencyManagement>
  <build>
    <pluginManagement>
      <plugins>
        <plugin>
          <groupId>org.apache.maven.plugins</groupId>
          <artifactId>maven-compiler-plugin</artifactId>
          <version>3.15.0</version>
        </plugin>
        <plugin>
          <groupId>org.apache.maven.plugins</groupId>
          <artifactId>maven-surefire-plugin</artifactId>
          <version>3.6.0</version>
        </plugin>
      </plugins>
    </pluginManagement>
    <plugins>
      <plugin>
        <groupId>org.apache.maven.plugins</groupId>
        <artifactId>maven-compiler-plugin</artifactId>
      </plugin>
      <plugin>
        <groupId>org.apache.maven.plugins</groupId>
        <artifactId>maven-surefire-plugin</artifactId>
      </plugin>
    </plugins>
  </build>
</project>
EOF
  cat > ci-lab/core/pom.xml << 'EOF'
<project xmlns="http://maven.apache.org/POM/4.0.0">
  <modelVersion>4.0.0</modelVersion>
  <parent>
    <groupId>io.pipelineiq.lab</groupId>
    <artifactId>pipelineiq-lab</artifactId>
    <version>0.1.0</version>
  </parent>
  <artifactId>core</artifactId>
  <dependencies>
    <dependency>
      <groupId>org.junit.jupiter</groupId>
      <artifactId>junit-jupiter</artifactId>
      <scope>test</scope>
    </dependency>
  </dependencies>
</project>
EOF
  cat > ci-lab/core/src/main/java/io/pipelineiq/lab/core/Core.java << 'EOF'
package io.pipelineiq.lab.core;

public final class Core {
    private Core() {}

    public static int add(int left, int right) {
        return left + right;
    }
}
EOF
  cat > ci-lab/core/src/test/java/io/pipelineiq/lab/core/CoreTest.java << 'EOF'
package io.pipelineiq.lab.core;

import static org.junit.jupiter.api.Assertions.assertEquals;

import org.junit.jupiter.api.Test;

class CoreTest {
    @Test
    void adds() {
        assertEquals(3, Core.add(1, 2));
    }
}
EOF
  cat > ci-lab/api/pom.xml << 'EOF'
<project xmlns="http://maven.apache.org/POM/4.0.0">
  <modelVersion>4.0.0</modelVersion>
  <parent>
    <groupId>io.pipelineiq.lab</groupId>
    <artifactId>pipelineiq-lab</artifactId>
    <version>0.1.0</version>
  </parent>
  <artifactId>api</artifactId>
  <dependencies>
    <dependency>
      <groupId>io.pipelineiq.lab</groupId>
      <artifactId>core</artifactId>
      <version>${project.version}</version>
    </dependency>
    <dependency>
      <groupId>org.junit.jupiter</groupId>
      <artifactId>junit-jupiter</artifactId>
      <scope>test</scope>
    </dependency>
  </dependencies>
</project>
EOF
  cat > ci-lab/api/src/main/java/io/pipelineiq/lab/api/Api.java << 'EOF'
package io.pipelineiq.lab.api;

import io.pipelineiq.lab.core.Core;

public final class Api {
    private Api() {}

    public static String greeting(String name) {
        return "hello " + name + " " + Core.add(1, 1);
    }
}
EOF
  cat > ci-lab/api/src/test/java/io/pipelineiq/lab/api/ApiTest.java << 'EOF'
package io.pipelineiq.lab.api;

import static org.junit.jupiter.api.Assertions.assertTrue;

import org.junit.jupiter.api.Test;

class ApiTest {
    @Test
    void serves() throws InterruptedException {
        Thread.sleep(1000);
        assertTrue(Api.greeting("lab").startsWith("hello lab"));
    }
}
EOF
  cat > ci-lab/flaky-lab/pom.xml << 'EOF'
<project xmlns="http://maven.apache.org/POM/4.0.0">
  <modelVersion>4.0.0</modelVersion>
  <parent>
    <groupId>io.pipelineiq.lab</groupId>
    <artifactId>pipelineiq-lab</artifactId>
    <version>0.1.0</version>
  </parent>
  <artifactId>flaky-lab</artifactId>
  <dependencies>
    <dependency>
      <groupId>org.junit.jupiter</groupId>
      <artifactId>junit-jupiter</artifactId>
      <scope>test</scope>
    </dependency>
  </dependencies>
</project>
EOF
  cat > ci-lab/flaky-lab/src/test/java/io/pipelineiq/lab/flaky/PlaceholderTest.java << 'EOF'
package io.pipelineiq.lab.flaky;

import static org.junit.jupiter.api.Assertions.assertTrue;

import org.junit.jupiter.api.Test;

class PlaceholderTest {
    @Test
    void holds() {
        assertTrue(true);
    }
}
EOF
  chmod -R a+rX ci-lab
}

dump_logs() {
  docker compose --env-file .env logs --tail 200 > p2-compose-logs.txt || true
  docker compose --env-file .env ps > p2-compose-ps.txt || true
}

trap dump_logs EXIT

cp .env.example .env
# A slash branch name is a poor library version. Point a plain ref at this commit.
git branch -f ci-library HEAD
sed -i 's|^PIPELINEIQ_LIBRARY_REF=.*|PIPELINEIQ_LIBRARY_REF=ci-library|' .env
write_lab

echo "starting the stack with tasks.ps1 up"
pwsh -NoProfile -Command '$env:COMPOSE_PATH_SEPARATOR=":"; $env:COMPOSE_FILE="docker-compose.yml:.github/compose.ci.yml"; & ./tasks.ps1 up'

echo "waiting for PipelineIQ and Jenkins"
for i in $(seq 1 90); do
  ready=$(curl -sS -o /tmp/readyz.out -w "%{http_code}" http://localhost:8080/readyz || echo curl-fail)
  login=$(curl -sS -o /tmp/login.out -w "%{http_code}" http://localhost:8081/login || echo curl-fail)
  if [ "$ready" = "200" ] && [ "$login" = "200" ]; then
    echo "PipelineIQ and Jenkins answered"
    break
  fi
  if [ $((i % 6)) -eq 0 ]; then
    echo "still waiting (${i}); readyz=${ready} login=${login}"
    docker compose --env-file .env ps || true
    echo "---- jenkins log ----"
    docker compose --env-file .env logs --tail 30 jenkins || true
    echo "---- server log ----"
    docker compose --env-file .env logs --tail 20 pipelineiq-server || true
  fi
  if [ "$i" -eq 90 ]; then
    echo "stack did not become ready; readyz=${ready} login=${login}" >&2
    echo "---- jenkins errors ----"
    docker compose --env-file .env logs jenkins 2>&1 | grep -E 'SEVERE|Error|ERROR|Failed|Exception|Configuration as Code|casc' | head -60 || true
    exit 1
  fi
  sleep 5
done

set -a
# shellcheck disable=SC1091
source .env
set +a
auth="${JENKINS_ADMIN_ID}:${JENKINS_ADMIN_PASSWORD}"

echo "waiting for the agent"
for i in $(seq 1 60); do
  body=$(curl -sS -u "$auth" http://localhost:8081/computer/agent/api/json || true)
  if printf '%s' "$body" | python3 -c 'import json,sys; d=json.load(sys.stdin); sys.exit(0 if d.get("offline") is False else 1)'; then
    echo "agent is online"
    break
  fi
  if [ "$i" -eq 60 ]; then
    echo "agent did not connect" >&2
    printf '%s\n' "$body" >&2
    exit 1
  fi
  sleep 5
done

PIPELINEIQ_TEST_DATABASE_URL="postgres://pipelineiq:${POSTGRES_PASSWORD}@localhost:5432/pipelineiq?sslmode=disable" \
  go test -race -count=1 -timeout 180s ./internal/ingest ./internal/db

crumb_json=$(curl -fsS -c /tmp/jcookies -b /tmp/jcookies -u "$auth" http://localhost:8081/crumbIssuer/api/json)
crumb_field=$(printf '%s' "$crumb_json" | python3 -c 'import json,sys; print(json.load(sys.stdin)["crumbRequestField"])')
crumb=$(printf '%s' "$crumb_json" | python3 -c 'import json,sys; print(json.load(sys.stdin)["crumb"])')

cat > p2-job.xml << 'EOF'
<?xml version='1.1' encoding='UTF-8'?>
<flow-definition plugin="workflow-job">
  <description>P2 verification build. Not a saved product job.</description>
  <keepDependencies>false</keepDependencies>
  <properties/>
  <definition class="org.jenkinsci.plugins.workflow.cps.CpsFlowDefinition" plugin="workflow-cps">
    <script><![CDATA[
@Library('pipelineiq') _
node('linux') {
  def result = 'SUCCESS'
  try {
    timedStage('Build and test') {
      sh 'cp -R /lab/. .'
      sh 'mvn -B test'
    }
  } catch (err) {
    result = 'FAILURE'
    throw err
  } finally {
    reportBuild(repository: 'nibinrj/pipelineiq-lab', branch: 'main', commit: 'ci-lab', result: result)
  }
}
]]></script>
    <sandbox>true</sandbox>
  </definition>
  <triggers/>
  <disabled>false</disabled>
</flow-definition>
EOF

code=$(curl -sS -c /tmp/jcookies -b /tmp/jcookies -o /tmp/create-job.out -w "%{http_code}" -u "$auth" \
  -H "${crumb_field}: ${crumb}" \
  -H "Content-Type: application/xml" \
  --data-binary @p2-job.xml \
  "http://localhost:8081/createItem?name=lab-manual")
if [ "$code" != "200" ]; then
  echo "createItem status ${code}" >&2
  cat /tmp/create-job.out >&2
  exit 1
fi

curl -fsS -c /tmp/jcookies -b /tmp/jcookies -u "$auth" -H "${crumb_field}: ${crumb}" -X POST \
  http://localhost:8081/job/lab-manual/build >/dev/null

echo "waiting for lab-manual"
for i in $(seq 1 60); do
  body=$(curl -sS -u "$auth" http://localhost:8081/job/lab-manual/lastBuild/api/json || true)
  if printf '%s' "$body" | python3 -c 'import json,sys; d=json.load(sys.stdin); sys.exit(0 if d.get("building") is False else 1)'; then
    printf '%s' "$body" | python3 -c 'import json,sys; d=json.load(sys.stdin); print("jenkins result", d.get("result"))'
    break
  fi
  if [ "$i" -eq 60 ]; then
    echo "lab build did not finish" >&2
    curl -fsS -u "$auth" http://localhost:8081/job/lab-manual/lastBuild/consoleText > p2-jenkins.log || true
    tail -80 p2-jenkins.log >&2 || true
    exit 1
  fi
  sleep 15
done

curl -fsS -u "$auth" http://localhost:8081/job/lab-manual/lastBuild/consoleText > p2-jenkins.log

build_id=$(docker compose --env-file .env exec -T postgres \
  psql -U pipelineiq -d pipelineiq -tAc "select id from build order by id desc limit 1" | tr -d '[:space:]')
if [ -z "$build_id" ]; then
  echo "no build row was ingested" >&2
  exit 1
fi

curl -fsS \
  -H "Authorization: Bearer ${PIPELINEIQ_INGEST_KEY}" \
  "http://localhost:8080/api/v1/builds/${build_id}" | python3 -m json.tool > p2-api.json
docker stats --no-stream > p2-stats.txt
docker compose --env-file .env ps > p2-compose-ps.txt
echo "API response:"
cat p2-api.json
echo "docker stats:"
cat p2-stats.txt

python3 - << 'PY'
import json
doc = json.load(open("p2-api.json"))
assert doc["repository"] == "nibinrj/pipelineiq-lab"
assert doc["result"] == "SUCCESS"
assert any(s["name"] == "Build and test" and s["result"] == "SUCCESS" for s in doc["stages"])
assert any(t["class_name"].endswith("CoreTest") and t["outcome"] == "PASSED" for t in doc["tests"])
assert any(t["module"] == "api" for t in doc["tests"])
print("lab build has stages and tests")
PY
