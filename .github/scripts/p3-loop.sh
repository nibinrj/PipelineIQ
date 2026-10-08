#!/usr/bin/env bash
# Twenty lab builds on the stack p2-stack.sh already started.
# One injected test is quarantined and later builds exclude it.
set -euo pipefail

root=$(cd "$(dirname "$0")/../.." && pwd)
cd "$root"

rm -rf ci-lab/flaky-lab
cp -a .github/lab-fixture/flaky-lab ci-lab/flaky-lab
chmod -R a+rX ci-lab

set -a
# shellcheck disable=SC1091
source .env
set +a
auth="${JENKINS_ADMIN_ID}:${JENKINS_ADMIN_PASSWORD}"

crumb_json=$(curl -fsS -c /tmp/jcookies -b /tmp/jcookies -u "$auth" http://localhost:8081/crumbIssuer/api/json)
crumb_field=$(printf '%s' "$crumb_json" | python3 -c 'import json,sys; print(json.load(sys.stdin)["crumbRequestField"])')
crumb=$(printf '%s' "$crumb_json" | python3 -c 'import json,sys; print(json.load(sys.stdin)["crumb"])')

cat > p3-job.xml << 'EOF'
<?xml version='1.1' encoding='UTF-8'?>
<flow-definition plugin="workflow-job">
  <description>P3 quarantine loop. Not a saved product job.</description>
  <keepDependencies>false</keepDependencies>
  <properties/>
  <definition class="org.jenkinsci.plugins.workflow.cps.CpsFlowDefinition" plugin="workflow-cps">
    <script><![CDATA[
@Library('pipelineiq') _
node('linux') {
  def result = 'SUCCESS'
  try {
    quarantineAwareTests(
      repository: 'nibinrj/pipelineiq-lab',
      mavenArgs: '-Dpipelineiq.flaky.epochSecond=100 -Dpipelineiq.flaky.orderSeed=1'
    )
  } catch (err) {
    result = 'FAILURE'
    throw err
  } finally {
    reportBuild(
      repository: 'nibinrj/pipelineiq-lab',
      branch: 'main',
      commit: 'ci-lab',
      result: result
    )
  }
}
]]></script>
    <sandbox>true</sandbox>
  </definition>
  <triggers/>
  <disabled>false</disabled>
</flow-definition>
EOF

code=$(curl -sS -c /tmp/jcookies -b /tmp/jcookies -o /tmp/p3-create.out -w "%{http_code}" -u "$auth" \
  -H "${crumb_field}: ${crumb}" \
  -H "Content-Type: application/xml" \
  --data-binary @p3-job.xml \
  "http://localhost:8081/createItem?name=lab-quarantine")
if [ "$code" != "200" ]; then
  echo "createItem status ${code}" >&2
  cat /tmp/p3-create.out >&2
  exit 1
fi

wait_build() {
  local want="$1"
  local i body number building
  for i in $(seq 1 40); do
    body=$(curl -sS -u "$auth" "http://localhost:8081/job/lab-quarantine/${want}/api/json" || true)
    if printf '%s' "$body" | python3 -c 'import json,sys; d=json.load(sys.stdin); sys.exit(0 if d.get("building") is False and d.get("result") else 1)'; then
      printf '%s' "$body" | python3 -c 'import json,sys; d=json.load(sys.stdin); print("build", d.get("number"), d.get("result"))'
      return 0
    fi
    sleep 15
  done
  echo "build ${want} did not finish" >&2
  curl -fsS -u "$auth" "http://localhost:8081/job/lab-quarantine/${want}/consoleText" > "p3-console-${want}.txt" || true
  tail -40 "p3-console-${want}.txt" >&2 || true
  return 1
}

for n in $(seq 1 20); do
  echo "starting lab-quarantine build ${n}"
  curl -fsS -c /tmp/jcookies -b /tmp/jcookies -u "$auth" -H "${crumb_field}: ${crumb}" -X POST \
    "http://localhost:8081/job/lab-quarantine/build" >/dev/null
  wait_build "$n"
done

curl -fsS -u "$auth" http://localhost:8081/job/lab-quarantine/12/consoleText > p3-console-12.txt
curl -fsS -u "$auth" http://localhost:8081/job/lab-quarantine/20/consoleText > p3-console-20.txt

curl -fsS \
  -H "Authorization: Bearer ${PIPELINEIQ_INGEST_KEY}" \
  "http://localhost:8080/api/v1/quarantine?repo=nibinrj/pipelineiq-lab" | python3 -m json.tool > p3-quarantine.json

build_id=$(docker compose --env-file .env exec -T postgres \
  psql -U pipelineiq -d pipelineiq -tAc "select id from build where job_name = 'lab-quarantine' order by build_number desc limit 1" | tr -d '[:space:]')
curl -fsS \
  -H "Authorization: Bearer ${PIPELINEIQ_INGEST_KEY}" \
  "http://localhost:8080/api/v1/builds/${build_id}" | python3 -m json.tool > p3-build.json

echo "quarantine API:"
cat p3-quarantine.json
echo "console lines from build 12:"
grep -E 'pipelineiq excludes|pipelineiq quarantine stage|failsAboutOneInFive|Tests run:' p3-console-12.txt | head -40 || true

python3 - << 'PY'
import json
doc = json.load(open("p3-quarantine.json"))
active = doc.get("active") or []
match = [item for item in active if item.get("method_name") == "failsAboutOneInFive"]
if not match:
    raise SystemExit("no quarantined random test: " + json.dumps(active))
console = open("p3-console-12.txt", errors="replace").read()
if "failsAboutOneInFive" not in console:
    raise SystemExit("build 12 console does not mention the quarantined test")
if "pipelineiq excludes" not in console:
    raise SystemExit("build 12 console does not show the excludes file")
print("quarantine loop excluded", match[0]["class_name"] + "#" + match[0]["method_name"], "by", match[0]["reason_rule"])
PY
